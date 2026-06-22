// Code generated — DO NOT EDIT.
import { addSolanaContractMock, type SolanaContractMock, type SolanaMock } from '@chainlink/cre-sdk/test'

import { {{.ProgramIDConst}} } from './{{.ClassName}}'

export type {{.MockName}} = SolanaContractMock

/**
 * Registers a {{.ClassName}} program mock on a SolanaMock instance.
 * The Solana CRE capability is write-only, so the mock routes writeReport
 * calls targeting this program's ID; set the returned mock's writeReport
 * property to define the reply.
 */
export function new{{.MockName}}(
  solanaMock: SolanaMock,
  programId: string | Uint8Array = {{.ProgramIDConst}},
): {{.MockName}} {
  return addSolanaContractMock(solanaMock, { programId })
}
