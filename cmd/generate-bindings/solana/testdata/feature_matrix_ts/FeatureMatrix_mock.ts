// Code generated — DO NOT EDIT.
import { addSolanaContractMock, type SolanaContractMock, type SolanaMock } from '@chainlink/cre-sdk/test'

import { FEATURE_MATRIX_PROGRAM_ID } from './FeatureMatrix'

export type FeatureMatrixMock = SolanaContractMock

/**
 * Registers a FeatureMatrix program mock on a SolanaMock instance.
 * The Solana CRE capability is write-only, so the mock routes writeReport
 * calls targeting this program's ID; set the returned mock's writeReport
 * property to define the reply.
 */
export function newFeatureMatrixMock(
  solanaMock: SolanaMock,
  programId: string | Uint8Array = FEATURE_MATRIX_PROGRAM_ID,
): FeatureMatrixMock {
  return addSolanaContractMock(solanaMock, { programId })
}
