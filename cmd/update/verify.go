package update

const (
	expectedSignerName  = "CRE"
	expectedSignerEmail = "cre@smartcontract.com"
	codesignIdentifier  = "com.smartcontract.cre.cli"
)

func getSigAssetName(platform, archName string) string {
	return "cre_" + platform + "_" + archName + ".sig"
}
