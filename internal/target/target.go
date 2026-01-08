package target

type Target struct {
	GOOS   string
	GOARCH string
	Zig    string
}

var Supported = []Target{
	{"linux", "amd64", "x86_64-linux-gnu"},
	{"linux", "arm64", "aarch64-linux-gnu"},
	{"linux", "386", "x86-linux-gnu"},
	{"linux", "arm", "arm-linux-gnueabihf"},
	{"linux", "riscv64", "riscv64-linux-gnu"},
	{"darwin", "amd64", "x86_64-macos"},
	{"darwin", "arm64", "aarch64-macos"},
	{"windows", "amd64", "x86_64-windows-gnu"},
	{"windows", "386", "x86-windows-gnu"},
	{"windows", "arm64", "aarch64-windows-gnu"},
	{"freebsd", "amd64", "x86_64-freebsd"},
}

func Find(goos, goarch string) *Target {
	for _, t := range Supported {
		if t.GOOS == goos && t.GOARCH == goarch {
			return &t
		}
	}
	return nil
}

func List() []string {
	result := make([]string, len(Supported))
	for i, t := range Supported {
		result[i] = t.GOOS + "/" + t.GOARCH
	}
	return result
}
