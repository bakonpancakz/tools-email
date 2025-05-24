package include

import _ "embed"

var (
	//go:embed content/robot.png
	NoreplyImage []byte

	//go:embed content/noreply.html
	NoreplyIndex string
)
