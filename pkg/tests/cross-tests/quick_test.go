package crosstests

import (
	"testing"

	"github.com/hexops/autogold/v2"
)

func TestQuick(t *testing.T) {
	q := qb()
	ty := q.objT().fld("f0", q.strT()).fld("f1", q.objT())
	value := q.obj().fld("f0", q.str("OK")).fld("f1", q.obj()).build(ty)
	autogold.Expect(`tftypes.Object["f0":tftypes.String, "f1":tftypes.Object[]]<"f0":tftypes.String<"OK">, "f1":tftypes.Object[]<>>`).Equal(t, value.String())
}
