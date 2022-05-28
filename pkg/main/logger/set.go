package logger

type StringSet struct {
	Values []string
}

func NewStringSet() StringSet {
	return StringSet{}
}

func NewStringSetMaxSize(size int) StringSet {
	return StringSet{Values: make([]string, 0, size)}
}
func NewStringSetExactSize(size int) StringSet {
	return StringSet{Values: make([]string, size)}
}

func (s *StringSet) Add(str string) {
	s.Values = append(s.Values, str)
}

func (s *StringSet) Length() int {
	return len(s.Values)
}

func (s *StringSet) Remove(str string) {
	new := s.Values[:0]
	for idx := range s.Values {
		if s.Values[idx] != str {
			new = append(new, s.Values[idx])
		}
	}
	s.Values = new
	new = nil
}

func (s *StringSet) Contains(str string) bool {
	for idx := range s.Values {
		if s.Values[idx] == str {
			return true
		}
	}
	return false
}

func (s *StringSet) Clear() {
	s.Values = nil
	s = nil
}

func (s *StringSet) Difference(dif StringSet) {
	new := s.Values[:0]
	for idx := range s.Values {
		insub := false
		for idx2 := range dif.Values {
			if s.Values[idx] == dif.Values[idx2] {
				insub = true
				break
			}
		}
		if !insub {
			new = append(new, s.Values[idx])
		}
	}
	s.Values = new
	new = nil
}

func (s *StringSet) Difference2(dif StringSet) {
	new := make([]string, 0, len(s.Values))
	for idx := range s.Values {
		insub := false
		for idx2 := range dif.Values {
			if s.Values[idx] == dif.Values[idx2] {
				insub = true
				break
			}
		}
		if !insub {
			new = append(new, s.Values[idx])
		}
	}
	s.Values = new
	new = nil
}

func (s *StringSet) Difference3(dif StringSet) {
	new := s.Values[:0]
	cont := make(map[string]struct{}, len(dif.Values))
	for idx := range dif.Values {
		cont[dif.Values[idx]] = struct{}{}
	}
	for idx := range s.Values {
		if _, ok := cont[s.Values[idx]]; !ok {
			new = append(new, s.Values[idx])
		}
	}
	s.Values = new
	new = nil
	cont = nil
}

func (s *StringSet) Union(add StringSet) {
	new := s.Values
	for idx := range add.Values {
		if !s.Contains(add.Values[idx]) {
			new = append(new, add.Values[idx])
		}
	}
	s.Values = new
	new = nil
}

type UIntSet struct {
	Values []uint32
}

func NewUintSet() UIntSet {
	return UIntSet{}
}

func NewUintSetMaxSize(size uint32) UIntSet {
	return UIntSet{Values: make([]uint32, 0, size)}
}
func NewUintSetExactSize(size uint32) UIntSet {
	return UIntSet{Values: make([]uint32, size)}
}

func (s *UIntSet) Add(val uint32) {
	s.Values = append(s.Values, val)
}

func (s *UIntSet) Length() int {
	return len(s.Values)
}

func (s *UIntSet) Remove(valchk uint32) {
	new := s.Values[:0]
	for idx := range s.Values {
		if s.Values[idx] != valchk {
			new = append(new, s.Values[idx])
		}
	}
	s.Values = new
	new = nil
}

func (s *UIntSet) Contains(valchk uint32) bool {
	for idx := range s.Values {
		if s.Values[idx] == valchk {
			return true
		}
	}
	return false
}

func (s *UIntSet) Clear() {
	s.Values = nil
	s = nil
}
