import { HTTPClient, consensusIdenticalAggregation, getNetwork, TxStatus } from "@chainlink/cre-sdk";
import { describe, expect } from "bun:test";
import {
  newTestRuntime,
  test,
  HttpActionsMock,
  EvmMock,
} from "@chainlink/cre-sdk/test";
import { initWorkflow, onCronTrigger, onLogTrigger, fetchReserveInfo } from "./main";
import type { Config } from "./main";
import type { Address } from "viem";
import { newBalanceReaderMock } from "./generated/BalanceReader_mock";
import { newIERC20Mock } from "./generated/IERC20_mock";
import { newMessageEmitterMock } from "./generated/MessageEmitter_mock";
import { newReserveManagerMock } from "./generated/ReserveManager_mock";

const mockConfig: Config = {
  schedule: "0 0 * * *",
  url: "https://example.com/api/por",
  evms: [
    {
      tokenAddress: "0x1234567890123456789012345678901234567890",
      porAddress: "0x2234567890123456789012345678901234567890",
      proxyAddress: "0x3234567890123456789012345678901234567890",
      balanceReaderAddress: "0x4234567890123456789012345678901234567890",
      messageEmitterAddress: "0x5234567890123456789012345678901234567890",
      chainSelectorName: "ethereum-testnet-sepolia",
      gasLimit: "1000000",
    },
  ],
};

/**
 * Helper to set up all EVM mocks for the PoR workflow.
 * Mocks three contract call paths:
 * 1. BalanceReader.getNativeBalances - returns mock native token balances
 * 2. IERC20.totalSupply - returns mock total supply
 * 3. MessageEmitter.getLastMessage - returns mock message (for log trigger)
 * 4. WriteReport - returns success for reserve updates
 */
const setupEVMMocks = (config: Config) => {
  const network = getNetwork({
    chainFamily: "evm",
    chainSelectorName: config.evms[0].chainSelectorName,
    isTestnet: true,
  });

  if (!network) {
    throw new Error(`Network not found for chain selector: ${config.evms[0].chainSelectorName}`);
  }

  const evmMock = EvmMock.testInstance(network.chainSelector.selector);

  // BalanceReader.getNativeBalances - returns mock native token balances (0.5 ETH in wei)
  const balanceMock = newBalanceReaderMock(config.evms[0].balanceReaderAddress as Address, evmMock);
  balanceMock.getNativeBalances = (addresses: readonly Address[]) => {
    expect(addresses.length).toBeGreaterThan(0);
    return addresses.map(() => 500000000000000000n);
  };

  // IERC20.totalSupply - returns mock total supply (1 token with 18 decimals)
  const erc20Mock = newIERC20Mock(config.evms[0].tokenAddress as Address, evmMock);
  erc20Mock.totalSupply = () => 1000000000000000000n;

  // MessageEmitter.getLastMessage - returns mock message (for log trigger)
  const messageMock = newMessageEmitterMock(config.evms[0].messageEmitterAddress as Address, evmMock);
  messageMock.getLastMessage = (emitter: Address) => {
    expect(emitter).toBeDefined();
    return "Test message from contract";
  };

  // ReserveManager - mock writeReport for updateReserves
  const reserveMock = newReserveManagerMock(config.evms[0].proxyAddress as Address, evmMock);
  reserveMock.writeReport = (req) => {
    expect(req.gasConfig.gasLimit?.toString()).toBe(config.evms[0].gasLimit);
    return {
      txStatus: TxStatus.SUCCESS,
      txHash: new Uint8Array(Buffer.from("1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", "hex")),
      errorMessage: "",
    };
  };

  return evmMock;
};

describe("fetchReserveInfo", () => {
  test("fetches and parses reserve info using HTTP capability", async () => {
    const runtime = newTestRuntime();
    runtime.config = mockConfig;

    const httpMock = HttpActionsMock.testInstance();

    const mockPORResponse = {
      accountName: "test-account",
      totalTrust: 1500000,
      totalToken: 1500000,
      ripcord: false,
      updatedAt: "2024-01-15T12:00:00Z",
    };

    httpMock.sendRequest = (req) => {
      expect(req.method).toBe("GET");
      expect(req.url).toBe(mockConfig.url);
      return {
        statusCode: 200,
        body: new TextEncoder().encode(JSON.stringify(mockPORResponse)),
        headers: {},
      };
    };

    const httpClient = new HTTPClient();
    const result = httpClient
      .sendRequest(runtime, fetchReserveInfo, consensusIdenticalAggregation())(mockConfig)
      .result();

    expect(result.totalReserve).toBe(mockPORResponse.totalToken);
    expect(result.lastUpdated).toBeInstanceOf(Date);
  });
});

describe("onCronTrigger", () => {
  test("executes full PoR workflow with all EVM calls", () => {
    const runtime = newTestRuntime();
    runtime.config = mockConfig;

    // Setup HTTP mock for reserve info
    const httpMock = HttpActionsMock.testInstance();
    const mockPORResponse = {
      accountName: "TrueUSD",
      totalTrust: 1000000,
      totalToken: 1000000,
      ripcord: false,
      updatedAt: "2023-01-01T00:00:00Z",
    };

    httpMock.sendRequest = (req) => {
      expect(req.method).toBe("GET");
      expect(req.url).toBe(mockConfig.url);
      return {
        statusCode: 200,
        body: new TextEncoder().encode(JSON.stringify(mockPORResponse)),
        headers: {},
      };
    };

    // Setup all EVM mocks
    setupEVMMocks(mockConfig);

    // Execute trigger with mock payload
    const result = onCronTrigger(runtime, {
      scheduledExecutionTime: {
        seconds: 1752514917n,
        nanos: 0,
      },
    });

    // Result should be the totalToken from mock response
    expect(result).toBeDefined();
    expect(typeof result).toBe("string");

    // Verify expected log messages were produced
    const logs = runtime.getLogs().map((log) => Buffer.from(log).toString("utf-8"));
    expect(logs.some((log) => log.includes("fetching por"))).toBe(true);
    expect(logs.some((log) => log.includes("ReserveInfo"))).toBe(true);
    expect(logs.some((log) => log.includes("TotalSupply"))).toBe(true);
    expect(logs.some((log) => log.includes("TotalReserveScaled"))).toBe(true);
    expect(logs.some((log) => log.includes("NativeTokenBalance"))).toBe(true);
  });

  test("validates scheduledExecutionTime is present", () => {
    const runtime = newTestRuntime();
    runtime.config = mockConfig;

    expect(() => onCronTrigger(runtime, {})).toThrow("Scheduled execution time is required");
  });
});

describe("onLogTrigger", () => {
  test("retrieves and returns message from contract", () => {
    const runtime = newTestRuntime();
    runtime.config = mockConfig;

    // Setup EVM mock for MessageEmitter
    setupEVMMocks(mockConfig);

    // Create mock EVMLog payload matching the expected structure
    // topics[1] should contain the emitter address (padded to 32 bytes)
    const mockLog = {
      topics: [
        Buffer.from("1234567890123456789012345678901234567890123456789012345678901234", "hex"),
        Buffer.from("000000000000000000000000abcdefabcdefabcdefabcdefabcdefabcdefabcd", "hex"),
        Buffer.from("000000000000000000000000000000000000000000000000000000006716eb80", "hex"),
      ],
      data: Buffer.from("", "hex"),
      blockNumber: { value: 100n },
    };

    const result = onLogTrigger(runtime, mockLog);

    expect(result).toBe("Test message from contract");

    // Verify log message
    const logs = runtime.getLogs().map((log) => Buffer.from(log).toString("utf-8"));
    expect(logs.some((log) => log.includes("Message retrieved from the contract"))).toBe(true);
  });
});

describe("initWorkflow", () => {
  test("returns two handlers with correct configuration", () => {
    const testSchedule = "*/10 * * * *";
    const config = { ...mockConfig, schedule: testSchedule };

    const handlers = initWorkflow(config);

    expect(handlers).toBeArray();
    expect(handlers).toHaveLength(2);
    expect(handlers[0].trigger.config.schedule).toBe(testSchedule);
    expect(handlers[0].fn.name).toBe("onCronTrigger");
    expect(handlers[1].trigger.config).toHaveProperty("addresses");
    expect(handlers[1].fn.name).toBe("onLogTrigger");
  });
});
