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


export const MessageEmitterABI = [{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"emitter","type":"address"},{"indexed":true,"internalType":"uint256","name":"timestamp","type":"uint256"},{"indexed":false,"internalType":"string","name":"message","type":"string"}],"name":"MessageEmitted","type":"event"},{"inputs":[{"internalType":"string","name":"message","type":"string"}],"name":"emitMessage","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address","name":"emitter","type":"address"}],"name":"getLastMessage","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"address","name":"emitter","type":"address"},{"internalType":"uint256","name":"timestamp","type":"uint256"}],"name":"getMessage","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"typeAndVersion","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"}] as const

export class MessageEmitter {
  constructor(
    private readonly client: EVMClient,
    public readonly address: Address,
  ) {}

  getLastMessage(
    runtime: Runtime<unknown>,
    emitter: `0x${string}`,
  ): string {
    const callData = encodeFunctionData({
      abi: MessageEmitterABI,
      functionName: 'getLastMessage' as const,
      args: [emitter],
    })

    const result = this.client
      .callContract(runtime, {
        call: encodeCallMsg({ from: zeroAddress, to: this.address, data: callData }),
        blockNumber: LAST_FINALIZED_BLOCK_NUMBER,
      })
      .result()

    return decodeFunctionResult({
      abi: MessageEmitterABI,
      functionName: 'getLastMessage' as const,
      data: bytesToHex(result.data),
    }) as string
  }

  getMessage(
    runtime: Runtime<unknown>,
    emitter: `0x${string}`,
    timestamp: bigint,
  ): string {
    const callData = encodeFunctionData({
      abi: MessageEmitterABI,
      functionName: 'getMessage' as const,
      args: [emitter, timestamp],
    })

    const result = this.client
      .callContract(runtime, {
        call: encodeCallMsg({ from: zeroAddress, to: this.address, data: callData }),
        blockNumber: LAST_FINALIZED_BLOCK_NUMBER,
      })
      .result()

    return decodeFunctionResult({
      abi: MessageEmitterABI,
      functionName: 'getMessage' as const,
      data: bytesToHex(result.data),
    }) as string
  }

  typeAndVersion(
    runtime: Runtime<unknown>,
  ): string {
    const callData = encodeFunctionData({
      abi: MessageEmitterABI,
      functionName: 'typeAndVersion' as const,
    })

    const result = this.client
      .callContract(runtime, {
        call: encodeCallMsg({ from: zeroAddress, to: this.address, data: callData }),
        blockNumber: LAST_FINALIZED_BLOCK_NUMBER,
      })
      .result()

    return decodeFunctionResult({
      abi: MessageEmitterABI,
      functionName: 'typeAndVersion' as const,
      data: bytesToHex(result.data),
    }) as string
  }

  writeReportFromEmitMessage(
    runtime: Runtime<unknown>,
    message: string,
    gasConfig?: { gasLimit?: string },
  ) {
    const callData = encodeFunctionData({
      abi: MessageEmitterABI,
      functionName: 'emitMessage' as const,
      args: [message],
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

