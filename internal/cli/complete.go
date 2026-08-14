package cli

import "strings"

var commands = []string{"/exit", "/quit", "/help", "/models", "/use", "/think", "/effort", "/context", "/tools", "/info"}

func Complete(line string, models []string) []string {
	fields := strings.Fields(line)
	switch len(fields) {
	case 0:
		return commands
	case 1:
		return prefixMatch(commands, fields[0])
	}
	arg := fields[1]
	switch fields[0] {
	case "/use":
		return prefixMatch(models, arg)
	case "/think":
		return prefixMatch([]string{"on", "off"}, arg)
	case "/effort":
		return prefixMatch([]string{"low", "high", "max"}, arg)
	}
	return nil
}

func prefixMatch(list []string, prefix string) []string {
	var out []string
	for _, s := range list {
		if strings.HasPrefix(s, prefix) {
			out = append(out, s)
		}
	}
	return out
}
