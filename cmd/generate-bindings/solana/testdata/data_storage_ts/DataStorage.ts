// Code generated — DO NOT EDIT.
import {
  addCodecSizePrefix,
  getArrayCodec,
  getBytesCodec,
  getStructCodec,
  getU32Codec,
  getU64Codec,
  getUtf8Codec,
} from "@solana/codecs";
import { getAddressCodec, type Address } from "@solana/addresses";
import {
  adaptTrigger,
  anchorCPILogTriggerConfig,
  bytesToBase64,
  bytesToHex,
  calculateAccountsHash,
  encodeBorshVecU32,
  encodeForwarderReport,
  prepareSolanaReportRequest,
  prepareSubkeyValue,
  type Runtime,
  type SolanaAccountMeta,
  SolanaClient,
  solanaAccountMetasToJson,
  solanaAddressToBytes,
  type SolanaComputeConfig,
  type SolanaDecodedLog,
  type SolanaFilterLogTriggerRequestJson,
  type SolanaLog,
  type SolanaLogTriggerOptions,
  type SolanaSubkeyConfigJson,
  type SolanaValueComparatorJson,
  type Trigger,
} from "@chainlink/cre-sdk";

export const DATA_STORAGE_PROGRAM_ID =
  "ECL8142j2YQAvs9R9geSsRnkVH2wLEi7soJCRyJ74cfL";

export const DATA_STORAGE_IDL = {
  address: "ECL8142j2YQAvs9R9geSsRnkVH2wLEi7soJCRyJ74cfL",
  metadata: {
    name: "data_storage",
    version: "0.1.0",
    spec: "0.1.0",
    description: "Created with Anchor",
  },
  instructions: [
    {
      name: "get_multiple_reserves",
      discriminator: [104, 122, 140, 104, 175, 151, 70, 42],
      accounts: [],
      args: [],
      returns: { vec: { defined: { name: "UpdateReserves" } } },
    },
    {
      name: "get_reserves",
      discriminator: [121, 140, 237, 84, 218, 105, 48, 17],
      accounts: [],
      args: [],
      returns: { defined: { name: "UpdateReserves" } },
    },
    {
      name: "get_tuple_reserves",
      discriminator: [189, 83, 186, 20, 127, 80, 109, 49],
      accounts: [],
      args: [],
    },
    {
      name: "initialize_data_account",
      discriminator: [9, 64, 78, 49, 71, 193, 15, 250],
      accounts: [
        {
          name: "data_account",
          writable: true,
          pda: {
            seeds: [
              {
                kind: "const",
                value: [100, 97, 116, 97, 95, 97, 99, 99, 111, 117, 110, 116],
              },
              { kind: "account", path: "user" },
            ],
          },
        },
        { name: "user", writable: true, signer: true },
        { name: "system_program", address: "11111111111111111111111111111111" },
      ],
      args: [{ name: "input", type: { defined: { name: "UserData" } } }],
    },
    {
      name: "log_access",
      discriminator: [196, 55, 194, 24, 5, 224, 161, 204],
      accounts: [{ name: "user", signer: true }],
      args: [{ name: "message", type: "string" }],
    },
    {
      name: "on_report",
      discriminator: [214, 173, 18, 221, 173, 148, 151, 208],
      accounts: [
        { name: "user", writable: true, signer: true },
        {
          name: "data_account",
          writable: true,
          pda: {
            seeds: [
              {
                kind: "const",
                value: [100, 97, 116, 97, 95, 97, 99, 99, 111, 117, 110, 116],
              },
              { kind: "account", path: "user" },
            ],
          },
        },
        { name: "system_program", address: "11111111111111111111111111111111" },
      ],
      args: [
        { name: "_metadata", type: "bytes" },
        { name: "payload", type: "bytes" },
      ],
    },
    {
      name: "update_key_value_data",
      discriminator: [67, 137, 144, 35, 210, 126, 254, 79],
      accounts: [
        { name: "user", writable: true, signer: true },
        {
          name: "data_account",
          writable: true,
          pda: {
            seeds: [
              {
                kind: "const",
                value: [100, 97, 116, 97, 95, 97, 99, 99, 111, 117, 110, 116],
              },
              { kind: "account", path: "user" },
            ],
          },
        },
      ],
      args: [
        { name: "key", type: "string" },
        { name: "value", type: "string" },
      ],
    },
    {
      name: "update_user_data",
      discriminator: [11, 13, 114, 150, 194, 224, 192, 78],
      accounts: [
        { name: "user", writable: true, signer: true },
        {
          name: "data_account",
          writable: true,
          pda: {
            seeds: [
              {
                kind: "const",
                value: [100, 97, 116, 97, 95, 97, 99, 99, 111, 117, 110, 116],
              },
              { kind: "account", path: "user" },
            ],
          },
        },
      ],
      args: [{ name: "input", type: { defined: { name: "UserData" } } }],
    },
  ],
  accounts: [
    { name: "DataAccount", discriminator: [85, 240, 182, 158, 76, 7, 18, 233] },
  ],
  events: [
    {
      name: "AccessLogged",
      discriminator: [243, 53, 225, 71, 64, 120, 109, 25],
    },
    {
      name: "DynamicEvent",
      discriminator: [236, 145, 224, 161, 9, 222, 218, 237],
    },
    { name: "NoFields", discriminator: [160, 156, 94, 85, 77, 122, 98, 240] },
  ],
  errors: [{ code: 6000, name: "DataNotFound", msg: "data not found" }],
  types: [
    {
      name: "AccessLogged",
      type: {
        kind: "struct",
        fields: [
          { name: "caller", type: "pubkey" },
          { name: "message", type: "string" },
        ],
      },
    },
    {
      name: "DataAccount",
      type: {
        kind: "struct",
        fields: [
          { name: "sender", type: "string" },
          { name: "key", type: "string" },
          { name: "value", type: "string" },
        ],
      },
    },
    {
      name: "DynamicEvent",
      type: {
        kind: "struct",
        fields: [
          { name: "key", type: "string" },
          { name: "user_data", type: { defined: { name: "UserData" } } },
          { name: "sender", type: "string" },
          { name: "metadata", type: "bytes" },
          { name: "metadata_array", type: { vec: "bytes" } },
        ],
      },
    },
    { name: "NoFields", type: { kind: "struct", fields: [] } },
    {
      name: "UpdateReserves",
      type: {
        kind: "struct",
        fields: [
          { name: "total_minted", type: "u64" },
          { name: "total_reserve", type: "u64" },
        ],
      },
    },
    {
      name: "UserData",
      type: {
        kind: "struct",
        fields: [
          { name: "key", type: "string" },
          { name: "value", type: "string" },
        ],
      },
    },
  ],
} as const;

// Base64 of the compact IDL JSON, passed to log triggers as contractIdlJson.
const DATA_STORAGE_IDL_BASE64 =
  "eyJhZGRyZXNzIjoiRUNMODE0MmoyWVFBdnM5UjlnZVNzUm5rVkgyd0xFaTdzb0pDUnlKNzRjZkwiLCJtZXRhZGF0YSI6eyJuYW1lIjoiZGF0YV9zdG9yYWdlIiwidmVyc2lvbiI6IjAuMS4wIiwic3BlYyI6IjAuMS4wIiwiZGVzY3JpcHRpb24iOiJDcmVhdGVkIHdpdGggQW5jaG9yIn0sImluc3RydWN0aW9ucyI6W3sibmFtZSI6ImdldF9tdWx0aXBsZV9yZXNlcnZlcyIsImRpc2NyaW1pbmF0b3IiOlsxMDQsMTIyLDE0MCwxMDQsMTc1LDE1MSw3MCw0Ml0sImFjY291bnRzIjpbXSwiYXJncyI6W10sInJldHVybnMiOnsidmVjIjp7ImRlZmluZWQiOnsibmFtZSI6IlVwZGF0ZVJlc2VydmVzIn19fX0seyJuYW1lIjoiZ2V0X3Jlc2VydmVzIiwiZGlzY3JpbWluYXRvciI6WzEyMSwxNDAsMjM3LDg0LDIxOCwxMDUsNDgsMTddLCJhY2NvdW50cyI6W10sImFyZ3MiOltdLCJyZXR1cm5zIjp7ImRlZmluZWQiOnsibmFtZSI6IlVwZGF0ZVJlc2VydmVzIn19fSx7Im5hbWUiOiJnZXRfdHVwbGVfcmVzZXJ2ZXMiLCJkaXNjcmltaW5hdG9yIjpbMTg5LDgzLDE4NiwyMCwxMjcsODAsMTA5LDQ5XSwiYWNjb3VudHMiOltdLCJhcmdzIjpbXX0seyJuYW1lIjoiaW5pdGlhbGl6ZV9kYXRhX2FjY291bnQiLCJkaXNjcmltaW5hdG9yIjpbOSw2NCw3OCw0OSw3MSwxOTMsMTUsMjUwXSwiYWNjb3VudHMiOlt7Im5hbWUiOiJkYXRhX2FjY291bnQiLCJ3cml0YWJsZSI6dHJ1ZSwicGRhIjp7InNlZWRzIjpbeyJraW5kIjoiY29uc3QiLCJ2YWx1ZSI6WzEwMCw5NywxMTYsOTcsOTUsOTcsOTksOTksMTExLDExNywxMTAsMTE2XX0seyJraW5kIjoiYWNjb3VudCIsInBhdGgiOiJ1c2VyIn1dfX0seyJuYW1lIjoidXNlciIsIndyaXRhYmxlIjp0cnVlLCJzaWduZXIiOnRydWV9LHsibmFtZSI6InN5c3RlbV9wcm9ncmFtIiwiYWRkcmVzcyI6IjExMTExMTExMTExMTExMTExMTExMTExMTExMTExMTExIn1dLCJhcmdzIjpbeyJuYW1lIjoiaW5wdXQiLCJ0eXBlIjp7ImRlZmluZWQiOnsibmFtZSI6IlVzZXJEYXRhIn19fV19LHsibmFtZSI6ImxvZ19hY2Nlc3MiLCJkaXNjcmltaW5hdG9yIjpbMTk2LDU1LDE5NCwyNCw1LDIyNCwxNjEsMjA0XSwiYWNjb3VudHMiOlt7Im5hbWUiOiJ1c2VyIiwic2lnbmVyIjp0cnVlfV0sImFyZ3MiOlt7Im5hbWUiOiJtZXNzYWdlIiwidHlwZSI6InN0cmluZyJ9XX0seyJuYW1lIjoib25fcmVwb3J0IiwiZGlzY3JpbWluYXRvciI6WzIxNCwxNzMsMTgsMjIxLDE3MywxNDgsMTUxLDIwOF0sImFjY291bnRzIjpbeyJuYW1lIjoidXNlciIsIndyaXRhYmxlIjp0cnVlLCJzaWduZXIiOnRydWV9LHsibmFtZSI6ImRhdGFfYWNjb3VudCIsIndyaXRhYmxlIjp0cnVlLCJwZGEiOnsic2VlZHMiOlt7ImtpbmQiOiJjb25zdCIsInZhbHVlIjpbMTAwLDk3LDExNiw5Nyw5NSw5Nyw5OSw5OSwxMTEsMTE3LDExMCwxMTZdfSx7ImtpbmQiOiJhY2NvdW50IiwicGF0aCI6InVzZXIifV19fSx7Im5hbWUiOiJzeXN0ZW1fcHJvZ3JhbSIsImFkZHJlc3MiOiIxMTExMTExMTExMTExMTExMTExMTExMTExMTExMTExMSJ9XSwiYXJncyI6W3sibmFtZSI6Il9tZXRhZGF0YSIsInR5cGUiOiJieXRlcyJ9LHsibmFtZSI6InBheWxvYWQiLCJ0eXBlIjoiYnl0ZXMifV19LHsibmFtZSI6InVwZGF0ZV9rZXlfdmFsdWVfZGF0YSIsImRpc2NyaW1pbmF0b3IiOls2NywxMzcsMTQ0LDM1LDIxMCwxMjYsMjU0LDc5XSwiYWNjb3VudHMiOlt7Im5hbWUiOiJ1c2VyIiwid3JpdGFibGUiOnRydWUsInNpZ25lciI6dHJ1ZX0seyJuYW1lIjoiZGF0YV9hY2NvdW50Iiwid3JpdGFibGUiOnRydWUsInBkYSI6eyJzZWVkcyI6W3sia2luZCI6ImNvbnN0IiwidmFsdWUiOlsxMDAsOTcsMTE2LDk3LDk1LDk3LDk5LDk5LDExMSwxMTcsMTEwLDExNl19LHsia2luZCI6ImFjY291bnQiLCJwYXRoIjoidXNlciJ9XX19XSwiYXJncyI6W3sibmFtZSI6ImtleSIsInR5cGUiOiJzdHJpbmcifSx7Im5hbWUiOiJ2YWx1ZSIsInR5cGUiOiJzdHJpbmcifV19LHsibmFtZSI6InVwZGF0ZV91c2VyX2RhdGEiLCJkaXNjcmltaW5hdG9yIjpbMTEsMTMsMTE0LDE1MCwxOTQsMjI0LDE5Miw3OF0sImFjY291bnRzIjpbeyJuYW1lIjoidXNlciIsIndyaXRhYmxlIjp0cnVlLCJzaWduZXIiOnRydWV9LHsibmFtZSI6ImRhdGFfYWNjb3VudCIsIndyaXRhYmxlIjp0cnVlLCJwZGEiOnsic2VlZHMiOlt7ImtpbmQiOiJjb25zdCIsInZhbHVlIjpbMTAwLDk3LDExNiw5Nyw5NSw5Nyw5OSw5OSwxMTEsMTE3LDExMCwxMTZdfSx7ImtpbmQiOiJhY2NvdW50IiwicGF0aCI6InVzZXIifV19fV0sImFyZ3MiOlt7Im5hbWUiOiJpbnB1dCIsInR5cGUiOnsiZGVmaW5lZCI6eyJuYW1lIjoiVXNlckRhdGEifX19XX1dLCJhY2NvdW50cyI6W3sibmFtZSI6IkRhdGFBY2NvdW50IiwiZGlzY3JpbWluYXRvciI6Wzg1LDI0MCwxODIsMTU4LDc2LDcsMTgsMjMzXX1dLCJldmVudHMiOlt7Im5hbWUiOiJBY2Nlc3NMb2dnZWQiLCJkaXNjcmltaW5hdG9yIjpbMjQzLDUzLDIyNSw3MSw2NCwxMjAsMTA5LDI1XX0seyJuYW1lIjoiRHluYW1pY0V2ZW50IiwiZGlzY3JpbWluYXRvciI6WzIzNiwxNDUsMjI0LDE2MSw5LDIyMiwyMTgsMjM3XX0seyJuYW1lIjoiTm9GaWVsZHMiLCJkaXNjcmltaW5hdG9yIjpbMTYwLDE1Niw5NCw4NSw3NywxMjIsOTgsMjQwXX1dLCJlcnJvcnMiOlt7ImNvZGUiOjYwMDAsIm5hbWUiOiJEYXRhTm90Rm91bmQiLCJtc2ciOiJkYXRhIG5vdCBmb3VuZCJ9XSwidHlwZXMiOlt7Im5hbWUiOiJBY2Nlc3NMb2dnZWQiLCJ0eXBlIjp7ImtpbmQiOiJzdHJ1Y3QiLCJmaWVsZHMiOlt7Im5hbWUiOiJjYWxsZXIiLCJ0eXBlIjoicHVia2V5In0seyJuYW1lIjoibWVzc2FnZSIsInR5cGUiOiJzdHJpbmcifV19fSx7Im5hbWUiOiJEYXRhQWNjb3VudCIsInR5cGUiOnsia2luZCI6InN0cnVjdCIsImZpZWxkcyI6W3sibmFtZSI6InNlbmRlciIsInR5cGUiOiJzdHJpbmcifSx7Im5hbWUiOiJrZXkiLCJ0eXBlIjoic3RyaW5nIn0seyJuYW1lIjoidmFsdWUiLCJ0eXBlIjoic3RyaW5nIn1dfX0seyJuYW1lIjoiRHluYW1pY0V2ZW50IiwidHlwZSI6eyJraW5kIjoic3RydWN0IiwiZmllbGRzIjpbeyJuYW1lIjoia2V5IiwidHlwZSI6InN0cmluZyJ9LHsibmFtZSI6InVzZXJfZGF0YSIsInR5cGUiOnsiZGVmaW5lZCI6eyJuYW1lIjoiVXNlckRhdGEifX19LHsibmFtZSI6InNlbmRlciIsInR5cGUiOiJzdHJpbmcifSx7Im5hbWUiOiJtZXRhZGF0YSIsInR5cGUiOiJieXRlcyJ9LHsibmFtZSI6Im1ldGFkYXRhX2FycmF5IiwidHlwZSI6eyJ2ZWMiOiJieXRlcyJ9fV19fSx7Im5hbWUiOiJOb0ZpZWxkcyIsInR5cGUiOnsia2luZCI6InN0cnVjdCIsImZpZWxkcyI6W119fSx7Im5hbWUiOiJVcGRhdGVSZXNlcnZlcyIsInR5cGUiOnsia2luZCI6InN0cnVjdCIsImZpZWxkcyI6W3sibmFtZSI6InRvdGFsX21pbnRlZCIsInR5cGUiOiJ1NjQifSx7Im5hbWUiOiJ0b3RhbF9yZXNlcnZlIiwidHlwZSI6InU2NCJ9XX19LHsibmFtZSI6IlVzZXJEYXRhIiwidHlwZSI6eyJraW5kIjoic3RydWN0IiwiZmllbGRzIjpbeyJuYW1lIjoia2V5IiwidHlwZSI6InN0cmluZyJ9LHsibmFtZSI6InZhbHVlIiwidHlwZSI6InN0cmluZyJ9XX19XX0=";

const DISCRIMINATOR_SIZE = 8;

const expectDiscriminator = (
  label: string,
  expected: Uint8Array,
  data: Uint8Array,
): Uint8Array => {
  if (data.length < DISCRIMINATOR_SIZE) {
    throw new Error(
      `${label}: data too short for discriminator (${data.length} bytes)`,
    );
  }
  for (let i = 0; i < DISCRIMINATOR_SIZE; i++) {
    if (data[i] !== expected[i]) {
      throw new Error(`${label}: discriminator mismatch`);
    }
  }
  return data.subarray(DISCRIMINATOR_SIZE);
};

export type AccessLogged = {
  caller: Address;
  message: string;
};

export const accessLoggedCodec = getStructCodec([
  ["caller", getAddressCodec()],
  ["message", addCodecSizePrefix(getUtf8Codec(), getU32Codec())],
]);

export type DataAccount = {
  sender: string;
  key: string;
  value: string;
};

export const dataAccountCodec = getStructCodec([
  ["sender", addCodecSizePrefix(getUtf8Codec(), getU32Codec())],
  ["key", addCodecSizePrefix(getUtf8Codec(), getU32Codec())],
  ["value", addCodecSizePrefix(getUtf8Codec(), getU32Codec())],
]);

export type UserData = {
  key: string;
  value: string;
};

export const userDataCodec = getStructCodec([
  ["key", addCodecSizePrefix(getUtf8Codec(), getU32Codec())],
  ["value", addCodecSizePrefix(getUtf8Codec(), getU32Codec())],
]);

export type DynamicEvent = {
  key: string;
  userData: UserData;
  sender: string;
  metadata: Uint8Array;
  metadataArray: Uint8Array[];
};

export const dynamicEventCodec = getStructCodec([
  ["key", addCodecSizePrefix(getUtf8Codec(), getU32Codec())],
  ["userData", userDataCodec],
  ["sender", addCodecSizePrefix(getUtf8Codec(), getU32Codec())],
  ["metadata", addCodecSizePrefix(getBytesCodec(), getU32Codec())],
  [
    "metadataArray",
    getArrayCodec(addCodecSizePrefix(getBytesCodec(), getU32Codec()), {
      size: getU32Codec(),
    }),
  ],
]);

export type NoFields = Record<string, never>;

export const noFieldsCodec = getStructCodec([]);

export type UpdateReserves = {
  totalMinted: bigint;
  totalReserve: bigint;
};

export const updateReservesCodec = getStructCodec([
  ["totalMinted", getU64Codec()],
  ["totalReserve", getU64Codec()],
]);

export const ACCOUNT_DATA_ACCOUNT_DISCRIMINATOR = new Uint8Array([
  85, 240, 182, 158, 76, 7, 18, 233,
]);

/**
 * Decodes raw DataAccount account data (with its 8-byte discriminator) into DataAccount.
 * Pure helper — there is no read capability; obtain the account bytes elsewhere.
 */
export const decodeDataAccountAccount = (data: Uint8Array): DataAccount =>
  dataAccountCodec.decode(
    expectDiscriminator(
      "account DataAccount",
      ACCOUNT_DATA_ACCOUNT_DISCRIMINATOR,
      data,
    ),
  ) as DataAccount;

export const EVENT_ACCESS_LOGGED_DISCRIMINATOR = new Uint8Array([
  243, 53, 225, 71, 64, 120, 109, 25,
]);

/**
 * Decodes raw AccessLogged event data (with its 8-byte discriminator) into AccessLogged.
 */
export const decodeAccessLoggedEvent = (data: Uint8Array): AccessLogged =>
  accessLoggedCodec.decode(
    expectDiscriminator(
      "event AccessLogged",
      EVENT_ACCESS_LOGGED_DISCRIMINATOR,
      data,
    ),
  ) as AccessLogged;

export const EVENT_DYNAMIC_EVENT_DISCRIMINATOR = new Uint8Array([
  236, 145, 224, 161, 9, 222, 218, 237,
]);

/**
 * Decodes raw DynamicEvent event data (with its 8-byte discriminator) into DynamicEvent.
 */
export const decodeDynamicEventEvent = (data: Uint8Array): DynamicEvent =>
  dynamicEventCodec.decode(
    expectDiscriminator(
      "event DynamicEvent",
      EVENT_DYNAMIC_EVENT_DISCRIMINATOR,
      data,
    ),
  ) as DynamicEvent;

export const EVENT_NO_FIELDS_DISCRIMINATOR = new Uint8Array([
  160, 156, 94, 85, 77, 122, 98, 240,
]);

/**
 * Decodes raw NoFields event data (with its 8-byte discriminator) into NoFields.
 */
export const decodeNoFieldsEvent = (data: Uint8Array): NoFields =>
  noFieldsCodec.decode(
    expectDiscriminator("event NoFields", EVENT_NO_FIELDS_DISCRIMINATOR, data),
  ) as NoFields;

export const parseAnyAccount = (data: Uint8Array): DataAccount => {
  const disc = data.subarray(0, DISCRIMINATOR_SIZE);
  const matches = (expected: Uint8Array) =>
    expected.every((b, i) => disc[i] === b);
  if (matches(ACCOUNT_DATA_ACCOUNT_DISCRIMINATOR))
    return decodeDataAccountAccount(data);
  throw new Error(
    `unknown account discriminator: [${Array.from(disc).join(", ")}]`,
  );
};

export const parseAnyEvent = (
  data: Uint8Array,
): AccessLogged | DynamicEvent | NoFields => {
  const disc = data.subarray(0, DISCRIMINATOR_SIZE);
  const matches = (expected: Uint8Array) =>
    expected.every((b, i) => disc[i] === b);
  if (matches(EVENT_ACCESS_LOGGED_DISCRIMINATOR))
    return decodeAccessLoggedEvent(data);
  if (matches(EVENT_DYNAMIC_EVENT_DISCRIMINATOR))
    return decodeDynamicEventEvent(data);
  if (matches(EVENT_NO_FIELDS_DISCRIMINATOR)) return decodeNoFieldsEvent(data);
  throw new Error(
    `unknown event discriminator: [${Array.from(disc).join(", ")}]`,
  );
};

/**
 * Optional filter values for AccessLogged log triggers. Set a field to filter on
 * that value (OR across filter rows). Leave unset for wildcard. Only top-level
 * scalar fields with supported subkey encodings are auto-filterable — nested
 * structs, vecs, arrays, bool, u128, and i128 need a manual SubkeyConfig.
 */
export type AccessLoggedFilters = {
  caller?: Address | null;
  message?: string | null;
};

export const encodeAccessLoggedSubkeys = (
  filters: AccessLoggedFilters[],
): SolanaSubkeyConfigJson[] => {
  const callerComparers: SolanaValueComparatorJson[] = [];
  const messageComparers: SolanaValueComparatorJson[] = [];
  for (const f of filters) {
    if (f.caller != null) {
      callerComparers.push({
        operator: "COMPARISON_OPERATOR_EQ",
        value: bytesToBase64(solanaAddressToBytes(f.caller)),
      });
    }
    if (f.message != null) {
      messageComparers.push({
        operator: "COMPARISON_OPERATOR_EQ",
        value: bytesToBase64(prepareSubkeyValue(f.message)),
      });
    }
  }
  const subkeys: SolanaSubkeyConfigJson[] = [];
  if (callerComparers.length > 0) {
    subkeys.push({ path: ["Caller"], comparers: callerComparers });
  }
  if (messageComparers.length > 0) {
    subkeys.push({ path: ["Message"], comparers: messageComparers });
  }
  return subkeys;
};

/**
 * Optional filter values for DynamicEvent log triggers. Set a field to filter on
 * that value (OR across filter rows). Leave unset for wildcard. Only top-level
 * scalar fields with supported subkey encodings are auto-filterable — nested
 * structs, vecs, arrays, bool, u128, and i128 need a manual SubkeyConfig.
 */
export type DynamicEventFilters = {
  key?: string | null;
  sender?: string | null;
  metadata?: Uint8Array | null;
};

export const encodeDynamicEventSubkeys = (
  filters: DynamicEventFilters[],
): SolanaSubkeyConfigJson[] => {
  const keyComparers: SolanaValueComparatorJson[] = [];
  const senderComparers: SolanaValueComparatorJson[] = [];
  const metadataComparers: SolanaValueComparatorJson[] = [];
  for (const f of filters) {
    if (f.key != null) {
      keyComparers.push({
        operator: "COMPARISON_OPERATOR_EQ",
        value: bytesToBase64(prepareSubkeyValue(f.key)),
      });
    }
    if (f.sender != null) {
      senderComparers.push({
        operator: "COMPARISON_OPERATOR_EQ",
        value: bytesToBase64(prepareSubkeyValue(f.sender)),
      });
    }
    if (f.metadata != null) {
      metadataComparers.push({
        operator: "COMPARISON_OPERATOR_EQ",
        value: bytesToBase64(prepareSubkeyValue(f.metadata)),
      });
    }
  }
  const subkeys: SolanaSubkeyConfigJson[] = [];
  if (keyComparers.length > 0) {
    subkeys.push({ path: ["Key"], comparers: keyComparers });
  }
  if (senderComparers.length > 0) {
    subkeys.push({ path: ["Sender"], comparers: senderComparers });
  }
  if (metadataComparers.length > 0) {
    subkeys.push({ path: ["Metadata"], comparers: metadataComparers });
  }
  return subkeys;
};

/**
 * Optional filter values for NoFields log triggers. Set a field to filter on
 * that value (OR across filter rows). Leave unset for wildcard. Only top-level
 * scalar fields with supported subkey encodings are auto-filterable — nested
 * structs, vecs, arrays, bool, u128, and i128 need a manual SubkeyConfig.
 */
export type NoFieldsFilters = Record<string, never>;

export const encodeNoFieldsSubkeys = (
  _filters: NoFieldsFilters[],
): SolanaSubkeyConfigJson[] => [];

export class DataStorage {
  readonly programId: Uint8Array;

  // The program ID is baked into the IDL, so it defaults to the generated
  // const — unlike EVM bindings where the address is a runtime value.
  constructor(
    private readonly client: SolanaClient,
    programId: string | Uint8Array = DATA_STORAGE_PROGRAM_ID,
  ) {
    this.programId =
      typeof programId === "string"
        ? solanaAddressToBytes(programId)
        : programId;
  }

  /**
   * Publishes a pre-encoded Borsh payload through the CRE signer to this
   * program's on_report entrypoint via the keystone-forwarder.
   *
   * remainingAccounts must follow the keystone-forwarder account layout:
   *   - Index 0: forwarderState – the forwarder program's state account.
   *   - Index 1: forwarderAuthority – PDA derived from seeds
   *     ["forwarder", forwarderState, receiverProgram] under the forwarder program ID.
   *   - Index 2+: receiver-specific accounts required by the target program.
   *
   * The full account list is hashed (via calculateAccountsHash) into the report.
   * The on-chain forwarder strips indices 0 and 1 before CPI-ing into the
   * receiver, so they must be present and correctly ordered.
   */
  writeReport(
    runtime: Runtime<unknown>,
    payload: Uint8Array,
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    const report = runtime
      .report(
        prepareSolanaReportRequest(
          encodeForwarderReport({
            accountHash: calculateAccountsHash(remainingAccounts),
            payload,
          }),
        ),
      )
      .result();

    return this.client
      .writeReport(runtime, {
        remainingAccounts: solanaAccountMetasToJson(remainingAccounts),
        receiver: bytesToHex(this.programId),
        computeConfig,
        report,
      })
      .result();
  }

  /**
   * Publishes a Borsh Vec of pre-encoded element payloads (mirrors Go's
   * WriteReportFromBorshEncodedVec). Each element must already be fully
   * serialized for one Vec item on the wire.
   */
  writeReportFromBorshEncodedVec(
    runtime: Runtime<unknown>,
    elementPayloads: Uint8Array[],
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReport(
      runtime,
      encodeBorshVecU32(elementPayloads),
      remainingAccounts,
      computeConfig,
    );
  }

  writeReportFromAccessLogged(
    runtime: Runtime<unknown>,
    input: AccessLogged,
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReport(
      runtime,
      new Uint8Array(accessLoggedCodec.encode(input)),
      remainingAccounts,
      computeConfig,
    );
  }

  writeReportFromAccessLoggeds(
    runtime: Runtime<unknown>,
    inputs: AccessLogged[],
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReportFromBorshEncodedVec(
      runtime,
      inputs.map((input) => new Uint8Array(accessLoggedCodec.encode(input))),
      remainingAccounts,
      computeConfig,
    );
  }

  writeReportFromDataAccount(
    runtime: Runtime<unknown>,
    input: DataAccount,
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReport(
      runtime,
      new Uint8Array(dataAccountCodec.encode(input)),
      remainingAccounts,
      computeConfig,
    );
  }

  writeReportFromDataAccounts(
    runtime: Runtime<unknown>,
    inputs: DataAccount[],
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReportFromBorshEncodedVec(
      runtime,
      inputs.map((input) => new Uint8Array(dataAccountCodec.encode(input))),
      remainingAccounts,
      computeConfig,
    );
  }

  writeReportFromUserData(
    runtime: Runtime<unknown>,
    input: UserData,
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReport(
      runtime,
      new Uint8Array(userDataCodec.encode(input)),
      remainingAccounts,
      computeConfig,
    );
  }

  writeReportFromUserDatas(
    runtime: Runtime<unknown>,
    inputs: UserData[],
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReportFromBorshEncodedVec(
      runtime,
      inputs.map((input) => new Uint8Array(userDataCodec.encode(input))),
      remainingAccounts,
      computeConfig,
    );
  }

  writeReportFromDynamicEvent(
    runtime: Runtime<unknown>,
    input: DynamicEvent,
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReport(
      runtime,
      new Uint8Array(dynamicEventCodec.encode(input)),
      remainingAccounts,
      computeConfig,
    );
  }

  writeReportFromDynamicEvents(
    runtime: Runtime<unknown>,
    inputs: DynamicEvent[],
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReportFromBorshEncodedVec(
      runtime,
      inputs.map((input) => new Uint8Array(dynamicEventCodec.encode(input))),
      remainingAccounts,
      computeConfig,
    );
  }

  writeReportFromNoFields(
    runtime: Runtime<unknown>,
    input: NoFields,
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReport(
      runtime,
      new Uint8Array(noFieldsCodec.encode(input)),
      remainingAccounts,
      computeConfig,
    );
  }

  writeReportFromNoFieldss(
    runtime: Runtime<unknown>,
    inputs: NoFields[],
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReportFromBorshEncodedVec(
      runtime,
      inputs.map((input) => new Uint8Array(noFieldsCodec.encode(input))),
      remainingAccounts,
      computeConfig,
    );
  }

  writeReportFromUpdateReserves(
    runtime: Runtime<unknown>,
    input: UpdateReserves,
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReport(
      runtime,
      new Uint8Array(updateReservesCodec.encode(input)),
      remainingAccounts,
      computeConfig,
    );
  }

  writeReportFromUpdateReservess(
    runtime: Runtime<unknown>,
    inputs: UpdateReserves[],
    remainingAccounts: SolanaAccountMeta[],
    computeConfig?: SolanaComputeConfig,
  ) {
    return this.writeReportFromBorshEncodedVec(
      runtime,
      inputs.map((input) => new Uint8Array(updateReservesCodec.encode(input))),
      remainingAccounts,
      computeConfig,
    );
  }

  /**
   * Registers a typed log trigger for AccessLogged events. The trigger
   * output is adapted to the decoded AccessLogged data alongside the raw log.
   * Pass opts.cpi for events emitted via Anchor's emit_cpi!.
   */
  logTriggerAccessLoggedLog(
    filterName: string,
    filters: AccessLoggedFilters[] = [],
    opts?: SolanaLogTriggerOptions,
  ): Trigger<SolanaLog, SolanaDecodedLog<AccessLogged>> {
    const config: SolanaFilterLogTriggerRequestJson = {
      name: filterName,
      address: bytesToBase64(this.programId),
      eventName: "AccessLogged",
      contractIdlJson: DATA_STORAGE_IDL_BASE64,
      subkeys: encodeAccessLoggedSubkeys(filters),
    };
    if (opts?.cpi) {
      config.cpiFilterConfig = anchorCPILogTriggerConfig(this.programId);
    }
    return adaptTrigger(this.client.logTrigger(config), (log) => ({
      log,
      data: decodeAccessLoggedEvent(log.data),
    }));
  }

  /**
   * Registers a typed log trigger for DynamicEvent events. The trigger
   * output is adapted to the decoded DynamicEvent data alongside the raw log.
   * Pass opts.cpi for events emitted via Anchor's emit_cpi!.
   */
  logTriggerDynamicEventLog(
    filterName: string,
    filters: DynamicEventFilters[] = [],
    opts?: SolanaLogTriggerOptions,
  ): Trigger<SolanaLog, SolanaDecodedLog<DynamicEvent>> {
    const config: SolanaFilterLogTriggerRequestJson = {
      name: filterName,
      address: bytesToBase64(this.programId),
      eventName: "DynamicEvent",
      contractIdlJson: DATA_STORAGE_IDL_BASE64,
      subkeys: encodeDynamicEventSubkeys(filters),
    };
    if (opts?.cpi) {
      config.cpiFilterConfig = anchorCPILogTriggerConfig(this.programId);
    }
    return adaptTrigger(this.client.logTrigger(config), (log) => ({
      log,
      data: decodeDynamicEventEvent(log.data),
    }));
  }

  /**
   * Registers a typed log trigger for NoFields events. The trigger
   * output is adapted to the decoded NoFields data alongside the raw log.
   * Pass opts.cpi for events emitted via Anchor's emit_cpi!.
   */
  logTriggerNoFieldsLog(
    filterName: string,
    filters: NoFieldsFilters[] = [],
    opts?: SolanaLogTriggerOptions,
  ): Trigger<SolanaLog, SolanaDecodedLog<NoFields>> {
    const config: SolanaFilterLogTriggerRequestJson = {
      name: filterName,
      address: bytesToBase64(this.programId),
      eventName: "NoFields",
      contractIdlJson: DATA_STORAGE_IDL_BASE64,
      subkeys: encodeNoFieldsSubkeys(filters),
    };
    if (opts?.cpi) {
      config.cpiFilterConfig = anchorCPILogTriggerConfig(this.programId);
    }
    return adaptTrigger(this.client.logTrigger(config), (log) => ({
      log,
      data: decodeNoFieldsEvent(log.data),
    }));
  }
}
