package ioc

import (
	"testing"
	"unsafe"
)

type IFoo interface{}

type Foo struct {
	A int
}

type Zoo struct {
	A string
	F IFoo `ioc:"required"`
}

type IBar interface {
	SetName(string)
	Name() string
}

type Bar struct {
	F1   IFoo `ioc:"required"`
	Zoo  *Zoo `ioc:"optional"`
	Xoo  *Xoo `ioc:"optional"`
	name string
}

func (b *Bar) SetName(s string) { b.name = s }
func (b *Bar) Name() string     { return b.name }

type Xoo struct {
}

func TestSingletonObject(t *testing.T) {
	RegistObject[Zoo](Transient)
	RegistInterface[IFoo, Foo](Singleton)
	ins1 := MustGetObject[IFoo]().(*Foo)
	if ins1 == nil {
		t.FailNow()
	}
	ins3 := MustGetObject[IFoo]().(*Foo)
	ins1.A = 100
	t.Logf("ins1:%v, value:%d", unsafe.Pointer(ins1), ins1.A)
	ins2 := MustGetObject[IFoo]().(*Foo)
	t.Logf("ins2:%v, value:%d", unsafe.Pointer(ins2), ins2.A)
	if ins2.A != 100 {
		t.FailNow()
	}
	t.Logf("ins3:%v, value:%d", unsafe.Pointer(ins3), ins3.A)
	if ins3.A != 100 {
		t.FailNow()
	}
	ins3.A = 300
	zoo := MustGetObject[Zoo]()
	if zoo.F.(*Foo).A != 300 {
		t.FailNow()
	}
	t.Logf("zoo:%+v", zoo)
}

func TestCreateObject(t *testing.T) {
	RegistObject[Zoo](Transient)
	RegistInterface[IFoo, Foo](Transient)
	RegistInterface[IBar, Bar](Transient)
	ins1 := MustGetObject[IFoo]().(*Foo)
	ins1.A = 100

	bar := MustGetObject[IBar]()

	if bar.(*Bar).Zoo == nil {
		t.FailNow()
	}

	if bar.(*Bar).Xoo != nil {
		t.FailNow()
	}

	t.Logf("bar1:%+v", bar)
	s := "hello world!!!"
	bar.SetName(s)
	if bar.Name() != s {
		t.FailNow()
	}
	t.Logf("name:%s", bar.Name())

	bar2 := MustGetObject[IBar]()
	unitPtr1 := unsafe.Pointer(bar.(*Bar).Zoo)
	unitPtr2 := unsafe.Pointer(bar2.(*Bar).Zoo)
	t.Logf("ptr1:%v, ptr2:%v", unitPtr1, unitPtr2)
	if unitPtr1 == unitPtr2 {
		t.FailNow()
	}
	t.Logf("bar2:%+v", bar2)
	t.Logf("name:%s", bar2.Name())

}

func TestFuncCreateObject(t *testing.T) {
	RegistObjectFunc[Foo](func() (*Foo, error) {
		return &Foo{A: 1000}, nil
	}, Singleton)

	foo1 := MustGetObject[*Foo]()
	foo2 := MustGetObject[*Foo]()
	if foo1.A != 1000 || foo1.A != foo2.A {
		t.FailNow()
	}
	unitPtr1 := unsafe.Pointer(foo1)
	unitPtr2 := unsafe.Pointer(foo2)
	if unitPtr1 != unitPtr2 {
		t.FailNow()
	}
	t.Logf("ptr1:%v, ptr2:%v", unitPtr1, unitPtr2)
}
func TestFuncCreateObject2(t *testing.T) {
	RegistObjectFunc[Foo](func() (*Foo, error) {
		return &Foo{A: 1000}, nil
	}, Transient)

	foo1 := MustGetObject[*Foo]()
	foo2 := MustGetObject[*Foo]()
	if foo1.A != 1000 || foo1.A != foo2.A {
		t.FailNow()
	}
	unitPtr1 := unsafe.Pointer(foo1)
	unitPtr2 := unsafe.Pointer(foo2)
	if unitPtr1 == unitPtr2 {
		t.FailNow()
	}
	t.Logf("ptr1:%v, ptr2:%v", unitPtr1, unitPtr2)

	RegistInterfaceFunc[IBar, Bar](func() (*Bar, error) {
		return &Bar{name: "hello"}, nil
	}, Transient)

	bar1 := MustGetObject[IBar]()
	if bar1.Name() != "hello" {
		t.FailNow()
	}
	bar1.SetName("world")
	bar2 := MustGetObject[IBar]()
	if bar2.Name() != "hello" {
		t.FailNow()
	}
}
