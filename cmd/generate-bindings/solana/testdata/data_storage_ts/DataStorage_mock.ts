// Code generated — DO NOT EDIT.
import { addSolanaContractMock, type SolanaContractMock, type SolanaMock } from '@chainlink/cre-sdk/test'

import { DATA_STORAGE_PROGRAM_ID } from './DataStorage'

export type DataStorageMock = SolanaContractMock

/**
 * Registers a DataStorage program mock on a SolanaMock instance.
 * The Solana CRE capability is write-only, so the mock routes writeReport
 * calls targeting this program's ID; set the returned mock's writeReport
 * property to define the reply.
 */
export function newDataStorageMock(
  solanaMock: SolanaMock,
  programId: string | Uint8Array = DATA_STORAGE_PROGRAM_ID,
): DataStorageMock {
  return addSolanaContractMock(solanaMock, { programId })
}
