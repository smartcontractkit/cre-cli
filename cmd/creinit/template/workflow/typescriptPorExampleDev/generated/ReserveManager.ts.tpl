// Code generated — DO NOT EDIT.
import { decodeFunctionResult, encodeFunctionData, zeroAddress } from 'viem'
import type { Address, Hex } from 'viem'
import {
  bytesToHex,
  encodeCallMsg,
  EVMClient,
  hexToBase64,
  LAST_FINALIZED_BLOCK_NUMBER,
  prepareReportRequest,
  type Runtime,
} from '@chainlink/cre-sdk'


export const ReserveManagerABI = [{"anonymous":false,"inputs":[{"indexed":false,"internalType":"uint256","name":"requestId","type":"uint256"}],"name":"RequestReserveUpdate","type":"event"},{"inputs":[],"name":"lastTotalMinted","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"lastTotalReserve","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"components":[{"internalType":"uint256","name":"totalMinted","type":"uint256"},{"internalType":"uint256","name":"totalReserve","type":"uint256"}],"internalType":"structUpdateReserves","name":"updateReserves","type":"tuple"}],"name":"updateReserves","outputs":[],"stateMutability":"nonpayable","type":"function"}] as const

export class ReserveManager {
  constructor(
    private readonly client: EVMClient,
    public readonly address: Address,
  ) {}

  lastTotalMinted(
    runtime: Runtime<unknown>,
  ): bigint {
    const callData = encodeFunctionData({
      abi: ReserveManagerABI,
      functionName: 'lastTotalMinted' as const,
    })

    const result = this.client
      .callContract(runtime, {
        call: encodeCallMsg({ from: zeroAddress, to: this.address, data: callData }),
        blockNumber: LAST_FINALIZED_BLOCK_NUMBER,
      })
      .result()

    return decodeFunctionResult({
      abi: ReserveManagerABI,
      functionName: 'lastTotalMinted' as const,
      data: bytesToHex(result.data),
    }) as bigint
  }

  lastTotalReserve(
    runtime: Runtime<unknown>,
  ): bigint {
    const callData = encodeFunctionData({
      abi: ReserveManagerABI,
      functionName: 'lastTotalReserve' as const,
    })

    const result = this.client
      .callContract(runtime, {
        call: encodeCallMsg({ from: zeroAddress, to: this.address, data: callData }),
        blockNumber: LAST_FINALIZED_BLOCK_NUMBER,
      })
      .result()

    return decodeFunctionResult({
      abi: ReserveManagerABI,
      functionName: 'lastTotalReserve' as const,
      data: bytesToHex(result.data),
    }) as bigint
  }

  writeReportFromUpdateReserves(
    runtime: Runtime<unknown>,
    updateReserves: { totalMinted: bigint; totalReserve: bigint },
    gasConfig?: { gasLimit?: string },
  ) {
    const callData = encodeFunctionData({
      abi: ReserveManagerABI,
      functionName: 'updateReserves' as const,
      args: [updateReserves],
    })

    const reportResponse = runtime
      .report(prepareReportRequest(callData))
      .result()

    return this.client
      .writeReport(runtime, {
        receiver: this.address,
        report: reportResponse,
        gasConfig,
      })
      .result()
  }

  writeReport(
    runtime: Runtime<unknown>,
    callData: Hex,
    gasConfig?: { gasLimit?: string },
  ) {
    const reportResponse = runtime
      .report(prepareReportRequest(callData))
      .result()

    return this.client
      .writeReport(runtime, {
        receiver: this.address,
        report: reportResponse,
        gasConfig,
      })
      .result()
  }
}

