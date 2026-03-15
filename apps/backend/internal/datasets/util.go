package datasets

func newLCG(seed uint64) *lcg { return &lcg{s: seed} }
