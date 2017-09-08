package resp

import (
	"testing"

	"github.com/stvp/assert"
)

func msg(content string) *Msg {
	return &Msg{data: []byte(content)}
}

func TestAnalysis(t *testing.T) {
	assert.Equal(t, msg("*1\r\n$7\r\nCOMMAND\r\n").Op(), MsgOpOther)
	assert.Equal(t, msg("*1\r\n$7\r\nAUTH\r\n").Op(), MsgOpOther)
	assert.Equal(t, msg("*2\r\n$7\r\nAUTH\r\n").Op(), MsgOpOther)
	assert.Equal(t, msg("*3\r\n$7\r\nAUTH\r\n$4\r\npass\r\n$14\r\nsomething-else\r\n").Op(), MsgOpOther)

	mAuth := msg("*2\r\n$4\r\nAUTH\r\n$4\r\npass\r\n")
	assert.Equal(t, mAuth.Op(), MsgOpAuth)
	assert.Equal(t, mAuth.FirstArg(), "pass")

	mAuthBroken := msg("*2\r\n$4\r\nAUTH\r\n$blah")
	assert.Equal(t, mAuthBroken.Op(), MsgOpBroken)
	assert.Equal(t, mAuthBroken.FirstArg(), "")

	mSelect := msg("*2\r\n$6\r\nSELECT\r\n$1\r\n2\r\n")
	assert.Equal(t, mSelect.Op(), MsgOpSelect)
	assert.Equal(t, mSelect.FirstArg(), "2")
	assert.Equal(t, mSelect.FirstArgInt(), 2)

	mSelectBroken := msg("*2\r\n$6\r\nSELECT\r\n$1\r\nX\r\n")
	assert.Equal(t, mSelectBroken.Op(), MsgOpBroken)
	assert.Equal(t, mSelectBroken.FirstArg(), "")

	mSync := msg("*1\r\n$4\r\nSYNC\r\n")
	assert.Equal(t, mSync.Op(), MsgOpSync)
	assert.Equal(t, mSync.FirstArg(), "")

	mPsync := msg("*3\r\n$5\r\nPSYNC\r\n$0\r\n\r\n$0\r\n\r\n")
	assert.Equal(t, mPsync.Op(), MsgOpPsync)
	assert.Equal(t, mPsync.FirstArg(), "")
}

func TestHelpers(t *testing.T) {
	assert.True(t, msg("+OK\r\n").IsOk())
	assert.False(t, msg("+OK\r").IsOk())
	assert.False(t, msg("-ERR some error\r\n").IsOk())
}
