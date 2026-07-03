package update

const (
	expectedSignerName  = "CRE"
	expectedSignerEmail = "cre@smartcontract.com"
	codesignIdentifier  = "com.smartcontract.cre.cli"
	// windowsSignerSubject is a substring of the Authenticode certificate subject.
	windowsSignerSubject = "Smart Contract"
)

func getSigAssetName(platform, archName string) string {
	return "cre_" + platform + "_" + archName + ".sig"
}
