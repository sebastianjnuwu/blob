package s3

import (
	"os"
	"testing"

	"blob/tests/e2e/testutil"
)

var suite *testutil.Ctx

func TestMain(m *testing.M) {
	suite = testutil.Setup("e2e-s3")
	code := m.Run()
	suite.Cleanup()
	os.Exit(code)
}
