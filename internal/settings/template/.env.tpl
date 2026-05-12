###############################################################################
### REQUIRED ENVIRONMENT VARIABLES - SENSITIVE INFORMATION                  ###
### DO NOT STORE RAW SECRETS HERE IN PLAINTEXT IF AVOIDABLE                 ###
### DO NOT UPLOAD OR SHARE THIS FILE UNDER ANY CIRCUMSTANCES                ###
###############################################################################
# Ethereum private key or 1Password reference (e.g. op://vault/item/field)
CRE_ETH_PRIVATE_KEY={{EthPrivateKey}}

# Aptos private key or 1Password reference (32-byte Ed25519 seed hex)
CRE_APTOS_PRIVATE_KEY={{AptosPrivateKey}}

# RPC secret keys — referenced in project.yaml via ${VAR_NAME} syntax.
# Example:
# CRE_SECRET_RPC_SEPOLIA=my-secret-api-key
# CRE_SECRET_RPC_MAINNET=my-other-api-key
