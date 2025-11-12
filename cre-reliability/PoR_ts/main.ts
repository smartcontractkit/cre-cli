import {
	bytesToHex,
	ConsensusAggregationByFields,
	type CronPayload,
	cre,
	encodeCallMsg,
	type HTTPSendRequester,
	hexToBase64,
    hexToBytes,
	LAST_FINALIZED_BLOCK_NUMBER,
	median,
    type EVMClient,
	Runner,
	type Runtime,
	TxStatus,
} from '@chainlink/cre-sdk'
import {Abi, type Address, decodeFunctionResult, encodeAbiParameters,encodePacked,encodeFunctionData, Hex, zeroAddress} from 'viem'
import { z } from 'zod'
import { BalanceReader} from './contracts/BalanceReader'
import {
    BalanceAtReply,
    BalanceAtRequest
} from "@chainlink/cre-sdk/dist/generated/capabilities/blockchain/evm/v1alpha/client_pb";

type PriceOutput = {
    feedId: string;     // string
    timestamp: number;  // uint32
    price: bigint;      // uint224
};

type PORResponse = {
    accountName: string;
    totalTrust: number; // float
    totalToken: number;
    ripcord: boolean;
    updatedAt: string;  // ISO
};

type TupleArrayReport = [Hex, number, BigInt][]

const onCronTrigger = (runtime: Runtime<Config>, payload: CronPayload): string => {
    let cfg:Config = runtime.config
    runtime.log("PoR_ts workflow started" + JSON.stringify(cfg));

    // Validate config (match Go’s checks)
    assertNonEmpty(cfg.schedule, "schedule", runtime);
    assertNonEmpty(cfg.url, "url", runtime);
    assertNonEmpty(cfg.balance_reader_address, "balance_reader_address", runtime);
    assertNonEmpty(cfg.address_one, "address_one", runtime);
    assertNonEmpty(cfg.address_two, "address_two", runtime);
    assertNonEmpty(cfg.data_feeds_cache_address, "data_feeds_cache_address", runtime);
    assertNonEmpty(cfg.feed_id, "feed_id", runtime);

    runtime.log("Config is valid")

    // Read balance #1 via eth_getBalance equivalent
    let evmClient = new cre.capabilities.EVMClient(BigInt("16015286601757825753"))

    runtime.log("evmClient created")

    let addressOneBalanceAtRequest:BalanceAtRequest={
        $typeName: "capabilities.blockchain.evm.v1alpha.BalanceAtRequest",
        account:hexToBytes(cfg.address_one),
    }
    let balanceAddressOne:BalanceAtReply = evmClient.balanceAt(runtime,addressOneBalanceAtRequest).result()
    // @ts-ignore
    runtime.log(`Got on-chain balance with BalanceAt() address: ${cfg.address_one} balance: ${balanceAddressOne.balance.toString()}` );

    // Read balance #2 via contract call: BalanceReader.getNativeBalances(address[])
    const callData = encodeFunctionData({
        abi: BalanceReader,
        functionName: 'getNativeBalances',
        args: [[cfg.address_two as Address]],
    })

    const contractCall = evmClient
        .callContract(runtime, {
            call: encodeCallMsg({
                from: zeroAddress,
                to: cfg.balance_reader_address as Address,
                data: callData,
            }),
            blockNumber: LAST_FINALIZED_BLOCK_NUMBER,
        })
        .result()
    runtime.log(`Got raw CallContract output: ${bytesToHex(contractCall.data)}`);


    // Decode the result
    const balances = decodeFunctionResult({
        abi: BalanceReader,
        functionName: 'getNativeBalances',
        data: bytesToHex(contractCall.data),
    })

    if (!balances || balances.length === 0) {
        throw new Error('No balances returned from contract')
    }

    const balances2:BigInt = balances[0]
    runtime.log(`Read on-chain balances (contract) from ${cfg.address_two} value:${balances2.toString()}`);

    const httpPrice = getHTTPPrice(cfg, runtime);

    runtime.log("Encoding report data")
    // Encode reports (tuple[] of (bytes32,uint32,uint224))
    const encoded = encodeReports(httpPrice,runtime);

    runtime.log("Encoding report")
    // Generate & write report on-chain
    const report =  runtime.report({
        encodedPayload: hexToBase64(encoded),
        encoderName:'evm',
        signingAlgo: 'ecdsa',
        hashingAlgo: 'keccak256',
    }).result();

    runtime.log("Writing report")
    const resp = evmClient
        .writeReport(runtime, {
            receiver: cfg.data_feeds_cache_address,
            report: report,
            gasConfig: {
                gasLimit: "5000000",
            },
        })
        .result()

    const txStatus = resp.txStatus

    if (txStatus !== TxStatus.SUCCESS) {
        throw new Error(`Failed to write report: ${resp.errorMessage || txStatus}`)
    }

    const txHash = resp.txHash || new Uint8Array(32)

    runtime.log(`Write report transaction succeeded at txHash: ${bytesToHex(txHash)}`)

    return txHash.toString()
}


function getHTTPPrice(cfg: Config,runtime:Runtime<Config>): PriceOutput {
    const httpCapability = new cre.capabilities.HTTPClient()

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

    return { feedId: cfg.feed_id , timestamp: Math.floor(reserveInfo.lastUpdated.getTime()/1000) , price: BigInt(reserveInfo.totalReserve * 1e18) };
}

const safeJsonStringify = (obj: any): string =>
    JSON.stringify(obj, (_, value) => (typeof value === 'bigint' ? value.toString() : value), 2)

interface ReserveInfo {
    lastUpdated: Date
    totalReserve: number
}

const fetchReserveInfo = (sendRequester: HTTPSendRequester, config: Config): ReserveInfo => {
    const response = sendRequester.sendRequest({ url: config.url, method:"GET" }).result()


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

function encodeReports(report: PriceOutput,runtime: Runtime<Config>): Hex {
    // tuple[] (bytes32 FeedID, uint32 Timestamp, uint224 Price)
    runtime.log(`Encoding ${report.feedId as Address} ${report.timestamp} ${report.price}`)

    let abiParameters = [
        {
            type: "tuple[]",
            components: [
                { type: "bytes32" },
                { type: "uint32" },
                { type: "uint224" },
            ]
        },
    ]
    let data: [Hex, number, BigInt][] = [[report.feedId as Address,report.timestamp,report.price]]

    let reps :Hex=encodeAbiParameters(abiParameters,[data])

    runtime.log(`Encoding done ${reps}`)
    return reps
}



function assertNonEmpty(v: string, name: string, runtime: Runtime<Config>) {
    if (!v || v.trim() === "") {
        runtime.log(`config value '${name}' cannot be empty`);
        throw new Error(`config value '${name}' cannot be empty`);
    }
}

const configSchema = z.object({
    schedule: z.string(),
    url: z.string(),
    balance_reader_address: z.string(),
    address_one: z.string(),
    address_two: z.string(),
    data_feeds_cache_address: z.string(),
    feed_id: z.string(),
})
type Config = z.infer<typeof configSchema>

export async function main() {
    const runner = await Runner.newRunner<Config>({
        configSchema,
    })
    await runner.run(initWorkflow)
}

const initWorkflow = (config: Config) => {
    const cronTrigger = new cre.capabilities.CronCapability()
    const cfg:Config = config

    return [
        cre.handler(
            cronTrigger.trigger({
                schedule: config.schedule,
            }),
            onCronTrigger,
        ),
    ]
}

main()