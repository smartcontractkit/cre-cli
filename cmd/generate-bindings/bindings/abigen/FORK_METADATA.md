# Abigen Fork Metadata

## Upstream Information

- Source Repository: https://github.com/ethereum/go-ethereum
- Original Package: accounts/abi/bind
- Fork Date: 2025-06-18
- Upstream Version: v1.16.0
- Upstream Commit: 4997a248ab4acdb40383f1e1a5d3813a634370a6

## Modifications

1. Custom Template Support (bindv2.go:300)
   - Description: Added `templateContent` parameter to `BindV2()` function signature
   - Reason: Enable CRE-specific binding generation with custom templates

2. isDynTopicType Function (bindv2.go:401-408)
   - Description: Added template function for event topic type checking
   - Registered `isDynTopicType` in the template function map
   - Reason: Distinguish hashed versus unhashed indexed event fields for dynamic types (tuples, strings, bytes, slices, arrays)

3. sanitizeStructNames Function (bindv2.go:383-395)
   - Reason: Generate cleaner, less verbose struct names in bindings
   - Description: Added function to remove contract name prefixes from struct names

4. Copyright Header Addition (bindv2.go:17-18)
   - Description: Added SmartContract ChainLink Limited SEZC copyright notice
   - Reason: Proper attribution for modifications

## Sync History

- 2025-06-18: Initial fork from v1.16.0

## Security Patches Applied

None yet.
