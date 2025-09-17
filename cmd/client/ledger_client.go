package client

type LedgerConfig struct {
	DerivationPath string
	LedgerEnabled  bool
}

//type LedgerClient struct {
//	logger         *zerolog.Logger
//	derivationPath []uint32
//	wallets        []accounts.Wallet
//	address        common.Address
//}

//func NewLedgerClient(logger *zerolog.Logger, derivationPath string) (*LedgerClient, error) {
//	derivationPathUint32, err := accounts.ParseDerivationPath(derivationPath)
//	if err != nil {
//		return nil, err
//	}
//	l := &LedgerClient{
//		logger:         logger,
//		derivationPath: derivationPathUint32,
//	}
//	if err := l.connectLedger(); err != nil {
//		return nil, err
//	}
//	return l, nil
//}
//
//func (l *LedgerClient) Address() common.Address {
//	return l.address
//}
//
//func (l *LedgerClient) loadAccount(index int) (*accounts.Account, func(), error) {
//	wallet := l.wallets[index]
//	l.logger.Debug().Interface("wallet", wallet).Int("index", index).Msg("Opening wallet")
//
//	// Open the ledger
//	if err := wallet.Open(""); err != nil {
//		return nil, func() {}, fmt.Errorf("failed to open wallet: %w", err)
//	}
//	l.logger.Debug().Interface("wallet", wallet).Int("index", index).Msg("Opened wallet")
//
//	// Load account
//	account, err := wallet.Derive(l.derivationPath, true)
//	if err != nil {
//		return nil, func() {}, fmt.Errorf("is your ledger ethereum app open? Failed to derive account: %w derivation path %v", err, l.derivationPath)
//	}
//	l.logger.Debug().Interface("account", account).Msg("Opened account")
//	return &account, func() { wallet.Close() }, nil
//}
//
//func (l *LedgerClient) connectLedger() error {
//	// Load ledger
//	ledgerhub, err := usbwallet.NewLedgerHub()
//	if err != nil {
//		return fmt.Errorf("failed to open ledger hub: %w", err)
//	}
//
//	// Get the first wallet
//	wallets := ledgerhub.Wallets()
//	if len(wallets) == 0 {
//		return errors.New("no wallets found")
//	}
//	l.wallets = wallets
//	l.logger.Debug().Int("num_wallets", len(wallets)).Msg("opened wallets")
//
//	account, closeAccount, err := l.loadAccount(0)
//	if err != nil {
//		return err
//	}
//	defer closeAccount()
//	l.address = account.Address
//	l.logger.Debug().Str("address", l.address.Hex()).Msg("Connected to ledger")
//
//	return nil
//}
//
//func (l *LedgerClient) SignTransactionWithLedger(tx *types.Transaction, chainID *big.Int) (*types.Transaction, error) {
//
//	l.logger.Debug().Interface("tx", tx).Msg("Signing transaction with ledger")
//	account, closeAccount, err := l.loadAccount(0)
//	if err != nil {
//		return nil, err
//	}
//	defer closeAccount()
//
//	l.logger.Info().Msgf("\n⚠️ Proceed on your ledger to SIGN the transaction on chain %d with nonce %d from address %s to address %s\n Data: %x", chainID, tx.Nonce(), account.Address.Hex(), tx.To().Hex(), tx.Data())
//	signTx, err := l.wallets[0].SignTx(*account, tx, chainID)
//	if err != nil {
//		return nil, err
//	}
//	l.logger.Info().Msgf("✅ Transaction with hash %s signed with ledger", signTx.Hash().Hex())
//	return signTx, nil
//}
