package common

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/test-go/testify/require"
)

var ZeroAddress = [32]byte{}

func MakeRandom32ByteArray() [32]byte {
	a := make([]byte, 32)
	if _, err := rand.Read(a); err != nil {
		panic(err) // should never panic but check in case
	}
	return [32]byte(a)
}

func Uint64ToLE(chain uint64) []byte {
	chainLE := make([]byte, 8)
	binary.LittleEndian.PutUint64(chainLE, chain)
	return chainLE
}

func ToPadded64Bytes(input []byte) (result [64]byte) {
	if len(input) > 64 {
		panic("input is too long")
	}
	copy(result[:], input[:])
	return result
}

func ToLeftPadded32Bytes(input []byte) (result [32]byte) {
	if len(input) > 32 {
		panic("input is too long")
	}
	start := 32 - len(input)
	copy(result[start:], input[:])
	return result
}

func NumToStr[T uint64 | uint32 | uint16 | uint8](num T) string {
	return fmt.Sprintf("%d", num)
}

func To28BytesLE(value uint64) [28]byte {
	le := make([]byte, 28)
	binary.LittleEndian.PutUint64(le, value)
	return [28]byte(le)
}

func To28BytesBE(value uint64) [28]byte {
	be := make([]byte, 28)
	binary.BigEndian.PutUint64(be[20:], value)
	return [28]byte(be)
}

func Map[T, V any](ts []T, fn func(T) V) []V {
	result := make([]V, len(ts))
	for i, t := range ts {
		result[i] = fn(t)
	}
	return result
}

func Discriminator(namespace, name string) []byte {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s:%s", namespace, name)))
	return h.Sum(nil)[:8]
}

func GetBlockTime(ctx context.Context, client *rpc.Client, commitment rpc.CommitmentType) (*solana.UnixTimeSeconds, error) {
	block, err := client.GetBlockHeight(ctx, commitment)
	if err != nil {
		return nil, fmt.Errorf("failed to get block height: %w", err)
	}

	blockTime, err := client.GetBlockTime(ctx, block)
	if err != nil {
		return nil, fmt.Errorf("failed to get block time: %w", err)
	}

	return blockTime, nil
}

type waitAndRetryOpts struct {
	RemainingAttempts uint
	Timeout           time.Duration
	Timestep          time.Duration
}

func (o waitAndRetryOpts) WithDecreasedAttempts() waitAndRetryOpts {
	return waitAndRetryOpts{
		RemainingAttempts: o.RemainingAttempts - 1,
		Timeout:           o.Timeout,
		Timestep:          o.Timestep,
	}
}

func FundAccounts(ctx context.Context, accounts []solana.PrivateKey, solanaGoClient *rpc.Client, t *testing.T) {
	fundAccounts(ctx, accounts, solanaGoClient, t, waitAndRetryOpts{
		RemainingAttempts: 5,
		Timeout:           30 * time.Second,
		Timestep:          500 * time.Millisecond,
	})
}
func fundAccounts(ctx context.Context, accounts []solana.PrivateKey, solanaGoClient *rpc.Client, t *testing.T, opts waitAndRetryOpts) {
	sigs := []solana.Signature{}
	for _, v := range accounts {
		sig, err := solanaGoClient.RequestAirdrop(ctx, v.PublicKey(), 1000*solana.LAMPORTS_PER_SOL, rpc.CommitmentFinalized)
		require.NoError(t, err)
		sigs = append(sigs, sig)
	}

	// wait for confirmation so later transactions don't fail
	remaining := accounts
	initTime := time.Now()
	for elapsed := time.Since(initTime); elapsed < opts.Timeout; elapsed = time.Since(initTime) {
		time.Sleep(opts.Timestep)

		statusRes, sigErr := solanaGoClient.GetSignatureStatuses(ctx, true, sigs...)
		require.NoError(t, sigErr)
		require.NotNil(t, statusRes)
		require.NotNil(t, statusRes.Value)

		accountsWithNonFinalizedFunding := []solana.PrivateKey{}
		for i, res := range statusRes.Value {
			if res == nil || res.ConfirmationStatus == rpc.ConfirmationStatusProcessed || res.ConfirmationStatus == rpc.ConfirmationStatusConfirmed {
				accountsWithNonFinalizedFunding = append(accountsWithNonFinalizedFunding, accounts[i])
			}
		}
		remaining = accountsWithNonFinalizedFunding

		if len(remaining) == 0 {
			return // all done!
		}
	}

	decreasedOpts := opts.WithDecreasedAttempts()
	if decreasedOpts.RemainingAttempts == 0 {
		require.NoError(t, fmt.Errorf("[%s]: unable to find transactions after all attempts", t.Name()))
	} else {
		fundAccounts(ctx, remaining, solanaGoClient, t, decreasedOpts) // recursive call with only remaining & with fewer attempts
	}
}
