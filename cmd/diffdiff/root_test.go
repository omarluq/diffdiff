package main_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	main "github.com/omarluq/diffdiff/cmd/diffdiff"
)

func TestRootCmd_ShowsHelp(t *testing.T) {
	t.Parallel()

	cmd := main.NewRootCmdForTest()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	// Use --help so the test exercises command wiring without launching the GUI
	// (a bare run opens the diff window and blocks indefinitely).
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "diffdiff")
}
