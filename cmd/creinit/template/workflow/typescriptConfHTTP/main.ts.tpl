import {
	type ConfidentialHTTPSendRequester,
	consensusIdenticalAggregation,
	handler,
	ConfidentialHTTPClient,
	CronCapability,
	json,
	ok,
	Runner,
	type Runtime,
	safeJsonStringify,
} from '@chainlink/cre-sdk'
import { z } from 'zod'

const configSchema = z.object({
	schedule: z.string(),
	owner: z.string(),
	url: z.string(),
})

type Config = z.infer<typeof configSchema>

type ResponseValues = {
	result: {
		headers: {
			'secret-header': string
		}
	}
}

const fetchResult = (sendRequester: ConfidentialHTTPSendRequester, config: Config) => {
	const { responses } = sendRequester
		.sendRequests({
			input: {
				requests: [
					{
						url: config.url,
						method: 'GET',
						headers: ['secret-header: {{.SECRET_HEADER}}'],
					},
				],
			},
			vaultDonSecrets: [
				{
					key: 'SECRET_HEADER',
					owner: config.owner,
				},
			],
		})
		.result()
	const response = responses[0]

	if (!ok(response)) {
		throw new Error(`HTTP request failed with status: ${response.statusCode}`)
	}

	return json(response) as ResponseValues
}

const onCronTrigger = (runtime: Runtime<Config>) => {
	runtime.log('Confidential HTTP workflow triggered.')

	const confHTTPClient = new ConfidentialHTTPClient()
	const result = confHTTPClient
		.sendRequests(
			runtime,
			fetchResult,
			consensusIdenticalAggregation(),
		)(runtime.config)
		.result()

	runtime.log(`Successfully fetched result: ${safeJsonStringify(result)}`)

	return {
		result,
	}
}

const initWorkflow = (config: Config) => {
	const cron = new CronCapability()

	return [handler(cron.trigger({ schedule: config.schedule }), onCronTrigger)]
}

export async function main() {
	const runner = await Runner.newRunner<Config>({ configSchema })

	await runner.run(initWorkflow)
}
