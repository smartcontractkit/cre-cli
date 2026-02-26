import {
	bytesToHex,
	ConsensusAggregationByFields,
	type CronPayload,
	handler,
	CronCapability,
	EVMClient,
	HTTPClient,
	type EVMLog,
	getNetwork,
	type HTTPSendRequester,
	median,
	Runner,
	type Runtime,
	TxStatus,
} from '@chainlink/cre-sdk'
import type { Address } from 'viem'
import { z } from 'zod'
import { BalanceReader } from '../contracts/evm/ts/generated/BalanceReader'
import { IERC20 } from '../contracts/evm/ts/generated/IERC20'
import { MessageEmitter } from '../contracts/evm/ts/generated/MessageEmitter'
import { ReserveManager } from '../contracts/evm/ts/generated/ReserveManager'

const configSchema = z.object({
	schedule: z.string(),
	url: z.string(),
	evms: z.array(
		z.object({
			tokenAddress: z.string(),
			porAddress: z.string(),
			proxyAddress: z.string(),
			balanceReaderAddress: z.string(),
			messageEmitterAddress: z.string(),
			chainSelectorName: z.string(),
			gasLimit: z.string(),
		}),
	),
})

export type Config = z.infer<typeof configSchema>

interface PORResponse {
	accountName: string
	totalTrust: number
	totalToken: number
	ripcord: boolean
	updatedAt: string
}

interface ReserveInfo {
	lastUpdated: Date
	totalReserve: number
}

const safeJsonStringify = (obj: any): string =>
	JSON.stringify(obj, (_, value) => (typeof value === 'bigint' ? value.toString() : value), 2)

export const fetchReserveInfo = (sendRequester: HTTPSendRequester, config: Config): ReserveInfo => {
	const response = sendRequester.sendRequest({ method: 'GET', url: config.url }).result()

	if (response.statusCode !== 200) {
		throw new Error(`HTTP request failed with status: ${response.statusCode}`)
	}

	const responseText = Buffer.from(response.body).toString('utf-8')
	const porResp: PORResponse = JSON.parse(responseText)

	if (porResp.ripcord) {
		throw new Error('ripcord is true')
	}

	return {
		lastUpdated: new Date(porResp.updatedAt),
		totalReserve: porResp.totalToken,
	}
}

const fetchNativeTokenBalance = (
	runtime: Runtime<Config>,
	evmConfig: Config['evms'][0],
	tokenHolderAddress: string,
): bigint => {
	const network = getNetwork({
		chainFamily: 'evm',
		chainSelectorName: evmConfig.chainSelectorName,
		isTestnet: true,
	})

	if (!network) {
		throw new Error(`Network not found for chain selector name: ${evmConfig.chainSelectorName}`)
	}

	const evmClient = new EVMClient(network.chainSelector.selector)
	const balanceReader = new BalanceReader(evmClient, evmConfig.balanceReaderAddress as Address)

	const balances = balanceReader.getNativeBalances(runtime, [tokenHolderAddress as Address])

	if (!balances || balances.length === 0) {
		throw new Error('No balances returned from contract')
	}

	return balances[0]
}

const getTotalSupply = (runtime: Runtime<Config>): bigint => {
	const evms = runtime.config.evms
	let totalSupply = 0n

	for (const evmConfig of evms) {
		const network = getNetwork({
			chainFamily: 'evm',
			chainSelectorName: evmConfig.chainSelectorName,
			isTestnet: true,
		})

		if (!network) {
			throw new Error(`Network not found for chain selector name: ${evmConfig.chainSelectorName}`)
		}

		const evmClient = new EVMClient(network.chainSelector.selector)
		const ierc20 = new IERC20(evmClient, evmConfig.tokenAddress as Address)

		const supply = ierc20.totalSupply(runtime)
		totalSupply += supply
	}

	return totalSupply
}

const updateReserves = (
	runtime: Runtime<Config>,
	totalSupply: bigint,
	totalReserveScaled: bigint,
): string => {
	const evmConfig = runtime.config.evms[0]
	const network = getNetwork({
		chainFamily: 'evm',
		chainSelectorName: evmConfig.chainSelectorName,
		isTestnet: true,
	})

	if (!network) {
		throw new Error(`Network not found for chain selector name: ${evmConfig.chainSelectorName}`)
	}

	const evmClient = new EVMClient(network.chainSelector.selector)
	const proxy = new ReserveManager(evmClient, evmConfig.proxyAddress as Address)

	runtime.log(
		`Updating reserves totalSupply ${totalSupply.toString()} totalReserveScaled ${totalReserveScaled.toString()}`,
	)

	const resp = proxy.writeReportFromUpdateReserves(
		runtime,
		{ totalMinted: totalSupply, totalReserve: totalReserveScaled },
		{ gasLimit: evmConfig.gasLimit },
	)

	const txStatus = resp.txStatus

	if (txStatus !== TxStatus.SUCCESS) {
		throw new Error(`Failed to write report: ${resp.errorMessage || txStatus}`)
	}

	const txHash = resp.txHash || new Uint8Array(32)

	runtime.log(`Write report transaction succeeded at txHash: ${bytesToHex(txHash)}`)

	return txHash.toString()
}

const doPOR = (runtime: Runtime<Config>): string => {
	runtime.log(`fetching por url ${runtime.config.url}`)

	const httpCapability = new HTTPClient()
	const reserveInfo = httpCapability
		.sendRequest(
			runtime,
			fetchReserveInfo,
			ConsensusAggregationByFields<ReserveInfo>({
				lastUpdated: median,
				totalReserve: median,
			}),
		)(runtime.config)
		.result()

	runtime.log(`ReserveInfo ${safeJsonStringify(reserveInfo)}`)

	const totalSupply = getTotalSupply(runtime)
	runtime.log(`TotalSupply ${totalSupply.toString()}`)

	const totalReserveScaled = BigInt(reserveInfo.totalReserve * 1e18)
	runtime.log(`TotalReserveScaled ${totalReserveScaled.toString()}`)

	const nativeTokenBalance = fetchNativeTokenBalance(
		runtime,
		runtime.config.evms[0],
		runtime.config.evms[0].tokenAddress,
	)
	runtime.log(`NativeTokenBalance ${nativeTokenBalance.toString()}`)

	updateReserves(runtime, totalSupply, totalReserveScaled)

	return reserveInfo.totalReserve.toString()
}

const getLastMessage = (
	runtime: Runtime<Config>,
	evmConfig: Config['evms'][0],
	emitter: string,
): string => {
	const network = getNetwork({
		chainFamily: 'evm',
		chainSelectorName: evmConfig.chainSelectorName,
		isTestnet: true,
	})

	if (!network) {
		throw new Error(`Network not found for chain selector name: ${evmConfig.chainSelectorName}`)
	}

	const evmClient = new EVMClient(network.chainSelector.selector)
	const messageEmitter = new MessageEmitter(evmClient, evmConfig.messageEmitterAddress as Address)

	return messageEmitter.getLastMessage(runtime, emitter as Address)
}

export const onCronTrigger = (runtime: Runtime<Config>, payload: CronPayload): string => {
	if (!payload.scheduledExecutionTime) {
		throw new Error('Scheduled execution time is required')
	}

	runtime.log('Running CronTrigger')

	return doPOR(runtime)
}

export const onLogTrigger = (runtime: Runtime<Config>, payload: EVMLog): string => {
	runtime.log('Running LogTrigger')

	const topics = payload.topics

	if (topics.length < 3) {
		runtime.log('Log payload does not contain enough topics')
		throw new Error(`log payload does not contain enough topics ${topics.length}`)
	}

	const emitter = bytesToHex(topics[1].slice(12))
	runtime.log(`Emitter ${emitter}`)

	const message = getLastMessage(runtime, runtime.config.evms[0], emitter)

	runtime.log(`Message retrieved from the contract ${message}`)

	return message
}

export const initWorkflow = (config: Config) => {
	const cronTrigger = new CronCapability()
	const network = getNetwork({
		chainFamily: 'evm',
		chainSelectorName: config.evms[0].chainSelectorName,
		isTestnet: true,
	})

	if (!network) {
		throw new Error(
			`Network not found for chain selector name: ${config.evms[0].chainSelectorName}`,
		)
	}

	const evmClient = new EVMClient(network.chainSelector.selector)

	return [
		handler(
			cronTrigger.trigger({
				schedule: config.schedule,
			}),
			onCronTrigger,
		),
		handler(
			evmClient.logTrigger({
				addresses: [config.evms[0].messageEmitterAddress],
			}),
			onLogTrigger,
		),
	]
}

export async function main() {
	const runner = await Runner.newRunner<Config>({
		configSchema,
	})
	await runner.run(initWorkflow)
}
