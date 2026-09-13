package services

import (
	"reflect"
	"testing"
)

func TestVillageFilterNames(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []string
	}{
		{"单选保持原行为", "应人石", []string{"应人石", "应人石社区"}},
		{"社区后缀展开", "应人石社区", []string{"应人石社区", "应人石"}},
		{"多选逗号分隔", "应人石,水田社区", []string{"应人石", "应人石社区", "水田社区", "水田"}},
		{"重复与空段去重", "应人石,,应人石, 应人石社区", []string{"应人石", "应人石社区"}},
		{"全空输入", ",, ,", []string{}},
	}
	for _, c := range cases {
		got := villageFilterNames(c.raw)
		if len(got) == 0 && len(c.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: villageFilterNames(%q) = %v, want %v", c.name, c.raw, got, c.want)
		}
	}
}
