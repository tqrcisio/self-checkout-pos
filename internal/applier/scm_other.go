//go:build !windows

package applier

import "fmt"

type SCM struct{}

func NewSCM() SCM { return SCM{} }

func (SCM) Stop(string) error  { return fmt.Errorf("scm: not supported on this platform") }
func (SCM) Start(string) error { return fmt.Errorf("scm: not supported on this platform") }
