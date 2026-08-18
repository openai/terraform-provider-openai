// Package acctest initializes safeguards before generated acceptance tests or
// terraform-plugin-testing can inspect the shared TF_ACC setting.
package acctest

import "os"

func init() {
	if err := enforceExactAcceptanceOptIn(os.LookupEnv, os.Unsetenv); err != nil {
		panic("failed to disable acceptance tests with an invalid TF_ACC setting: " + err.Error())
	}
}

// Keep this guard handwritten: acceptance.go is owned by the upstream generator.
func enforceExactAcceptanceOptIn(
	lookupEnv func(string) (string, bool),
	unsetEnv func(string) error,
) error {
	value, exists := lookupEnv("TF_ACC")
	if !exists || value == "1" {
		return nil
	}

	// terraform-plugin-testing treats any nonempty TF_ACC as acceptance opt-in.
	return unsetEnv("TF_ACC")
}
