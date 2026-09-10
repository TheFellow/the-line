package track

import (
	"strings"
	"testing"
)

func TestImportCSV(t *testing.T) {
	valid := "width_right,x,bank,z,y,width_left,surface\n7,0,8,2,0,4,asphalt\n8,100,10,4,0,5,wet\n"
	s, err := ImportCSV(strings.NewReader(valid), "Survey with permission")
	if err != nil {
		t.Fatal(err)
	}
	if s.Version != 2 || s.Points[1].WidthLeft != 5 || s.Points[1].WidthRight != 8 || s.Points[1].Surface != "wet" {
		t.Fatalf("wrong import %+v", s)
	}
	for _, bad := range []string{
		"x,y,z,width_left\n0,0,0,6\n",
		"x,y,z,width_left,width_right,width_right\n",
		"x,y,z,width_left,width_right,unknown\n",
		"x,y,z,width_left,width_right\nNaN,0,0,6,6\n100,0,0,6,6\n",
		"x,y,z,width_left,width_right\n0,0,0,1,6\n100,0,0,6,6\n",
		"x,y,z,width_left,width_right\n0,0,0,6,6\n0,0,0,6,6\n",
		"x,y,z,width_left,width_right\n0,0,0,6\n",
	} {
		if _, err := ImportCSV(strings.NewReader(bad), "bad"); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func TestImportRejectsOversizeTail(t *testing.T) {
	valid := "x,y,z,width_left,width_right\n0,0,0,6,6\n100,0,0,6,6\n"
	if _, err := ImportCSV(strings.NewReader(valid+strings.Repeat("\n", 2<<20)), "large"); err == nil {
		t.Fatal("oversize trailing bytes accepted")
	}
}
