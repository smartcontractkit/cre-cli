# cre-cli

## 1.9.0

### Minor Changes

- [#566](https://github.com/smartcontractkit/cre-cli/pull/566) [`7276be0`](https://github.com/smartcontractkit/cre-cli/commit/7276be0eb801037fd04527c990f75f0e745c013d) Thanks [@russell-stern](https://github.com/russell-stern)! - Bump cre-sdk-go to v1.21.0 and @chainlink/cre-sdk to ^1.22.0

### Patch Changes

- [#607](https://github.com/smartcontractkit/cre-cli/pull/607) [`29b8ebb`](https://github.com/smartcontractkit/cre-cli/commit/29b8ebb7b14c12486396fde646143cc62d97960c) Thanks [@ejacquier](https://github.com/ejacquier)! - Fix telemetry for commands that don't require login (`cre generate-bindings`, `cre workflow build`, `cre workflow hash`, etc.). Credentials are now attached silently when they exist on disk, so usage events from logged-in users are sent instead of being dropped. Commands remain fully usable while logged out.

## 1.8.2

### Patch Changes

- [#332](https://github.com/smartcontractkit/cre-cli/pull/332) [`31c1ab8`](https://github.com/smartcontractkit/cre-cli/commit/31c1ab8a500fa8ad1518ea610628302ba5ee76f2) Thanks [@timothyF95](https://github.com/timothyF95)! - Submit oauth secrets to vault DON

## 1.8.1

### Patch Changes

- [#333](https://github.com/smartcontractkit/cre-cli/pull/333) [`e6c2be1`](https://github.com/smartcontractkit/cre-cli/commit/e6c2be1c8ec1dfb635698b6731bbf11ca7c9ee67) Thanks [@timothyF95](https://github.com/timothyF95)! - Changeset test PR
