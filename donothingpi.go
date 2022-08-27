/*
donothingpi
Implements vattu interface. Minimal implementation for testing software.
Also acts as simple example
*/
package govattu

type DoNothingPi struct {
}

func (p *DoNothingPi) PwmSetMode(en0 bool, ms0 bool, en1 bool, ms1 bool) {}
func (p *DoNothingPi) Pwm0SetRange(ra uint32)                            {}
func (p *DoNothingPi) Pwm1SetRange(ra uint32)                            {}
func (p *DoNothingPi) Pwm0Set(r uint32)                                  {}
func (p *DoNothingPi) Pwm1Set(r uint32)                                  {}
func (p *DoNothingPi) SetPWM0Hi()                                        {}
func (p *DoNothingPi) SetPWM0Lo()                                        {}
func (p *DoNothingPi) SetToHwPWM0(rf *RfSettings)                        {}
func (p *DoNothingPi) PwmSetClock(divisor uint32)                        {}
func (p *DoNothingPi) Close() error                                      { return nil }

func (p *DoNothingPi) PinClear(pin uint8)                                 {}
func (p *DoNothingPi) PinSet(pin uint8)                                   {}
func (p *DoNothingPi) PinMode(pin uint8, alt AltSetting)                  {}
func (p *DoNothingPi) ReadPinLevel(pin uint8) bool                        { return false }
func (p *DoNothingPi) ReadAllPinLevels() uint64                           { return 0 }
func (p *DoNothingPi) ReadPinEvent(pin uint8) bool                        { return false }
func (p *DoNothingPi) ReadAllPinEvents() uint64                           { return 0 }
func (p *DoNothingPi) ResetPinEvent(pin uint8)                            {}
func (p *DoNothingPi) ZeroPinEventDetectMask()                            {}
func (p *DoNothingPi) SetPinEventDetectMask(pin uint8, pine PinEventMask) {}
func (p *DoNothingPi) PullMode(pin uint8, pull PinPull)                   {}
