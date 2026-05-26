package chain

import "testing"

func TestParseTriggerChainSelector(t *testing.T) {
	tests := []struct {
		name   string
		id     string
		prefix string
		want   uint64
		ok     bool
	}{
		{
			name:   "evm mainnet",
			id:     "evm:ChainSelector:5009297550715157269@1.0.0 LogTrigger",
			prefix: "evm",
			want:   uint64(5009297550715157269),
			ok:     true,
		},
		{
			name:   "aptos mainnet",
			id:     "aptos:ChainSelector:4741433654826277614@1.0.0",
			prefix: "aptos",
			want:   uint64(4741433654826277614),
			ok:     true,
		},
		{
			name:   "evm lowercase",
			id:     "evm:chainselector:16015286601757825753@1.0.0",
			prefix: "evm",
			want:   uint64(16015286601757825753),
			ok:     true,
		},
		{
			name:   "uppercase",
			id:     "EVM:CHAINSELECTOR:16015286601757825753@1.0.0",
			prefix: "evm",
			want:   uint64(16015286601757825753),
			ok:     true,
		},
		{
			name:   "leading and trailing spaces",
			id:     "   evm:ChainSelector:123@1.0.0   ",
			prefix: "evm",
			want:   uint64(123),
			ok:     true,
		},
		{
			name:   "prefix mismatch - evm parser sees aptos id",
			id:     "aptos:ChainSelector:123@1.0.0",
			prefix: "evm",
			want:   0,
			ok:     false,
		},
		{
			name:   "prefix mismatch - aptos parser sees evm id",
			id:     "evm:ChainSelector:123@1.0.0",
			prefix: "aptos",
			want:   0,
			ok:     false,
		},
		{
			name:   "no selector present",
			id:     "evm@1.0.0 LogTrigger",
			prefix: "evm",
			want:   0,
			ok:     false,
		},
		{
			name:   "non-numeric selector",
			id:     "evm:ChainSelector:notanumber@1.0.0",
			prefix: "evm",
			want:   0,
			ok:     false,
		},
		{
			name:   "empty selector",
			id:     "evm:ChainSelector:@1.0.0",
			prefix: "evm",
			want:   0,
			ok:     false,
		},
		{
			name:   "overflow uint64",
			id:     "evm:ChainSelector:18446744073709551616@1.0.0",
			prefix: "evm",
			want:   0,
			ok:     false,
		},
		{
			name:   "digits followed by letters (regex grabs only digits)",
			id:     "evm:ChainSelector:987abc@1.0.0",
			prefix: "evm",
			want:   uint64(987),
			ok:     true,
		},
		{
			name:   "prefix must be at start - embedded prefix rejected",
			id:     "foo evm:ChainSelector:1 bar",
			prefix: "evm",
			want:   0,
			ok:     false,
		},
		{
			name:   "zero selector",
			id:     "evm:ChainSelector:0@1.0.0",
			prefix: "evm",
			want:   0,
			ok:     true,
		},
		{
			name:   "max uint64",
			id:     "evm:ChainSelector:18446744073709551615@1.0.0",
			prefix: "evm",
			want:   uint64(18446744073709551615),
			ok:     true,
		},
		{
			name:   "negative sign not matched",
			id:     "evm:ChainSelector:-1@1.0.0",
			prefix: "evm",
			want:   0,
			ok:     false,
		},
		{
			name:   "unicode digits rejected",
			id:     "evm:ChainSelector:１２３@1.0.0",
			prefix: "evm",
			want:   0,
			ok:     false,
		},
		{
			name:   "tab before number rejected",
			id:     "evm:ChainSelector:\t42@1.0.0",
			prefix: "evm",
			want:   0,
			ok:     false,
		},
		{
			name:   "empty prefix rejected",
			id:     "evm:ChainSelector:1@1.0.0",
			prefix: "",
			want:   0,
			ok:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseTriggerChainSelector(tt.prefix, tt.id)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("ParseTriggerChainSelector(%q, %q) = (%d, %v); want (%d, %v)", tt.prefix, tt.id, got, ok, tt.want, tt.ok)
			}
		})
	}
}
