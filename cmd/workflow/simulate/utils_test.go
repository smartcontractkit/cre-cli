package simulate

import "testing"

func TestParseChainSelectorFromTriggerID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want uint64
		ok   bool
	}{
		{
			name: "mainnet format",
			id:   "evm:ChainSelector:5009297550715157269@1.0.0 LogTrigger",
			want: uint64(5009297550715157269),
			ok:   true,
		},
		{
			name: "sepolia lowercase",
			id:   "evm:chainselector:16015286601757825753@1.0.0",
			want: uint64(16015286601757825753),
			ok:   true,
		},
		{
			name: "sepolia uppercase",
			id:   "EVM:CHAINSELECTOR:16015286601757825753@1.0.0",
			want: uint64(16015286601757825753),
			ok:   true,
		},
		{
			name: "leading and trailing spaces",
			id:   "   evm:ChainSelector:123@1.0.0   ",
			want: uint64(123),
			ok:   true,
		},
		{
			name: "no selector present",
			id:   "evm@1.0.0 LogTrigger",
			want: 0,
			ok:   false,
		},
		{
			name: "non-numeric selector",
			id:   "evm:ChainSelector:notanumber@1.0.0",
			want: 0,
			ok:   false,
		},
		{
			name: "empty selector",
			id:   "evm:ChainSelector:@1.0.0",
			want: 0,
			ok:   false,
		},
		{
			name: "overflow uint64",
			// 2^64 is overflow for uint64 (max is 2^64-1)
			id:   "evm:ChainSelector:18446744073709551616@1.0.0",
			want: 0,
			ok:   false,
		},
		{
			name: "digits followed by letters (regex grabs only digits)",
			id:   "evm:ChainSelector:987abc@1.0.0",
			want: uint64(987),
			ok:   true,
		},
		{
			name: "multiple occurrences - returns first",
			id:   "foo ChainSelector:1 bar ChainSelector:2 baz",
			want: uint64(1),
			ok:   true,
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseChainSelectorFromTriggerID(tt.id)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("parseChainSelectorFromTriggerID(%q) = (%d, %v); want (%d, %v)", tt.id, got, ok, tt.want, tt.ok)
			}
		})
	}
}
