package tfdiags

import (
	"encoding/gob"
)

type internalRpcFriendlyDiag struct {
	Severity_ Severity
	Summary_  string
	Detail_   string
	Subject_  *SourceRange
	Context_  *SourceRange
}

// internalRpcFriendlyDiag transforms a given diagnostic so that is more friendly to
// RPC.
//
// In particular, it currently returns an object that can be serialized and
// later re-inflated using gob. This definition may grow to include other
// serializations later.
func makeRPCFriendlyDiag(diag Diagnostic) Diagnostic {
	desc := diag.Description()
	source := diag.Source()
	return &internalRpcFriendlyDiag{
		Severity_: diag.Severity(),
		Summary_:  desc.Summary,
		Detail_:   desc.Detail,
		Subject_:  source.Subject,
		Context_:  source.Context,
	}
}

func (d *internalRpcFriendlyDiag) Severity() Severity {
	return d.Severity_
}

func (d *internalRpcFriendlyDiag) Description() Description {
	return Description{
		Summary: d.Summary_,
		Detail:  d.Detail_,
	}
}

func (d *internalRpcFriendlyDiag) Source() Source {
	return Source{
		Subject: d.Subject_,
		Context: d.Context_,
	}
}

func (d internalRpcFriendlyDiag) FromExpr() *FromExpr {
	// RPC-friendly diagnostics cannot preserve expression information because
	// expressions themselves are not RPC-friendly.
	return nil
}

func (d internalRpcFriendlyDiag) ExtraInfo() interface{} {
	// RPC-friendly diagnostics always discard any "extra information".
	return nil
}

func init() {
	gob.Register((*internalRpcFriendlyDiag)(nil))
}
