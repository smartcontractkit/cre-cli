import { timestampNow, type Timestamp } from '@bufbuild/protobuf/wkt'
import { cre, type CronPayload, type HTTPPayload, type EVMLog } from "@chainlink/cre-sdk/cre";
import { Value } from "@chainlink/cre-sdk/utils/values/value";
import type { Runtime, NodeRuntime } from "@chainlink/cre-sdk/runtime/runtime";
import { consensusMedianAggregation, hexToBytes } from '@chainlink/cre-sdk/utils'

type Config = {
  schedule: string;
  url: string;
  evms: {
    tokenAddress: string;
    porAddress: string;
    balanceReaderAddress: string;
    messageEmitterAddress: string;
    chainSelector: string;
    gasLimit: number;
  }[];
};

async function fetchReserveInfo(_: NodeRuntime, config: Config) {
  const response = await cre.utils.fetch({
    url: config.url,
  });

  return JSON.parse(response.body);
}

async function fetchNativeTokenBalance(config: Config, runtime: Runtime, tokenHolderAddress: string) {
  const evmCfg = config.evms[0]
  const logger = runtime.logger
  const evmClient = new cre.capabilities.EVMClient(undefined, BigInt(evmCfg.chainSelector))

  // TODO: Do we need a hexToAddress util?
  const balanceReaderAddress = hexToBytes(evmCfg.balanceReaderAddress)
  // balanceReader, err := balance_reader.NewBalanceReader(evmClient, balanceReaderAddress, nil) TODO: Where do we get a BalanceReader?
  // const balanceReader = new BalanceReader(evmClient, balanceReaderAddress)
  const tokenAddress = hexToBytes(tokenHolderAddress)
  // const balances = balanceReader.getNativeBalances(runtime, {addresses: [tokenAddress]});

  // return balances[0];

  return 0;
}

async function getTotalSupply(config: Config, runtime: Runtime) {
  const evms = config.evms
  const logger = runtime.logger
  let totalSupply = 0n
 
  for (const evmCfg of evms) {
    const evmClient = new cre.capabilities.EVMClient(undefined, BigInt(evmCfg.chainSelector))
    // TODO: Do we need hexToAddress util?
    const address = hexToBytes(evmCfg.tokenAddress)
    // const token = ierc20.NewIERC20(evmClient, address) // TODO: How do we do this?
    // const supply = await token.TotalSupply(runtime, big.NewInt(8771643))
    // 
    // totalSupply += supply;
  }

  return totalSupply;
}

async function updateReserves(config: Config, runtime: Runtime, totalSupply: bigint, totalReserveScaled: bigint) {
  const logger = runtime.logger
  const evmCfg = config.evms[0]
  const evmClient = new cre.capabilities.EVMClient(undefined, BigInt(evmCfg.chainSelector))

  logger.log(`Updating reserves totalSupply ${totalSupply} totalReserveScaled ${totalReserveScaled}`)

  // TODO: How do we generate a ReserveManager?
  // reserveManager, err := reserve_manager.NewReserveManager(evmClient, common.HexToAddress(evmCfg.ProxyAddress), nil)
  // 	resp, err := reserveManager.WriteReportFromUpdateReserves(runtime, reserve_manager.UpdateReserves{
  // 		TotalMinted:  totalSupply,
  // 		TotalReserve: totalReserveScaled,
  // 	}, nil).Await()
}

async function doPOR(config: Config, runtime: Runtime, time: Timestamp) {
  const logger = runtime.logger;

  logger.log(`fetching por url ${config.url}`)

  const reserveInfo = await cre.runInNodeMode(
    fetchReserveInfo,
    // consensusAggregationFromTags() // TODO: How do we do this from tags? 
    consensusMedianAggregation()
  )(config);

  logger.log(`ReserveInfo ${JSON.stringify(reserveInfo)}`);

  const totalSupply = await getTotalSupply(config, runtime);

  logger.log(`TotalSupply ${totalSupply}`);

  // totalReserveScaled := reserveInfo.TotalReserve.Mul(decimal.NewFromUint64(1e18)).BigInt() // TODO get totalReserve
  const totalReserveScaled = 0n

  logger.log(`TotalReserveScaled ${totalReserveScaled}`)

  const nativeTokenBalance = await fetchNativeTokenBalance(config, runtime, config.evms[0].tokenAddress);

  logger.log(`NativeTokenBalance ${nativeTokenBalance}`)

  const secretAddress = await runtime.getSecret('SECRET_ADDRESS')  
  const secretAddressBalance = await fetchNativeTokenBalance(config, runtime, secretAddress)

  logger.log(`SecretAddressBalance ${secretAddressBalance}`)

  updateReserves(config, runtime, totalSupply, totalReserveScaled)

	// Update reserves
	// if err := updateReserves(config, runtime, totalSupply, totalReserveScaled); err != nil {
	// 	return "", fmt.Errorf("failed to update reserves: %w", err)
	// }

	// return reserveInfo.TotalReserve.String(), nil

  cre.sendResponseValue(Value.from(`TotalReserve 0`));
}

function prepareMessageEmitter(config: Config, runtime: Runtime) {
  const evmCfg = config.evms[0];
  const evmClient = new cre.capabilities.EVMClient(undefined, BigInt(evmCfg.chainSelector));
  const address = hexToBytes(evmCfg.messageEmitterAddress) // TODO: do we need a hexToAddress helper?

  // TODO: Where do we get a MessageEmitter
  // return new MessageEmitter(evmClient, address);
}

async function onCronTrigger(config: Config, runtime: Runtime, payload: CronPayload) {
  runtime.logger.log('Running CronTrigger');

  return doPOR(config, runtime, payload.scheduledExecutionTime);
};

async function onLogTrigger(config: Config, runtime: Runtime, payload: EVMLog) {
  const logger = runtime.logger;
  
  logger.log("Running LogTrigger");

  const messageEmitter = prepareMessageEmitter(config, runtime);
  const topics = payload.topics;

  if (topics.length < 3) {
		logger.log("Log payload does not contain enough topics")
		throw new Error(`log payload does not contain enough topics ${topics.length}`);
	}

  const emitter = topics[1].slice(12)
  logger.log(`Emitter ${emitter}`)
  // lastMessageInput := message_emitter.GetLastMessageInput{
	// 	Emitter: common.Address(emitter),
	// }
  // const lastMessageInput = messageEmitter.getLastMessageInput({
  //   emitter: address(emitter); // TODO: Do we need an address helper?
  // });
	// const message = messageEmitter.getLastMessage(runtime, lastMessageInput, 8771643n);
  const message = 'FAKE_MESSAGE'

  logger.log(`Message retrieved from the contract ${message}`)

  cre.sendResponseValue(Value.from(message));
}

async function onHTTPTrigger(config: Config, runtime: Runtime, payload: HTTPPayload) {
  // TODO: Figure out why this won't run without erroring out
  const logger = runtime.logger;

  logger.log("Raw HTTP trigger received")

  cre.sendResponseValue(Value.from('FINISHED HTTP TRIGGER'));
}

// ./http_trigger_payload.json

function initWorkflow(config: Config) {
  const cronTrigger = new cre.capabilities.CronCapability();
  const httpTrigger = new cre.capabilities.HTTPCapability();
  const evmClient = new cre.capabilities.EVMClient(undefined, BigInt(config.evms[0].chainSelector));

  return [
    cre.handler(
      cronTrigger.trigger({
        schedule: config.schedule,
      }),
      onCronTrigger,
    ),
    cre.handler(
      evmClient.logTrigger({
        addresses: [config.evms[0].messageEmitterAddress],
      }),
      onLogTrigger,
    ),
    cre.handler(
      httpTrigger.trigger({}),
      onHTTPTrigger,
    ),
  ];
};

export async function main() {
  const runner = await cre.newRunner();

  await runner.run(initWorkflow);
}

main();
