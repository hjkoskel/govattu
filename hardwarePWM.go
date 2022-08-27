/*
Utilities for hardware PWM control

Functions for calculating

*/
package govattu

import (
	"fmt"
	"math"
	"time"
)

const PWMCLOCKNS float64 = 52.0833333333 //=(1000*1000*1000)/19200000  104.1666667 / 2.0 //One "block" takes this long
const MINPWMC uint32 = 2
const MAXPWMC uint32 = 4095 //2090
const MAXPWMR uint32 = 4096

type RfSettings struct {
	Pwmc uint32 //Range 2-2090
	Pwmr uint32 //TODO range
	Pwm  uint32 //TODO range
}

func (p *RfSettings) Equal(a RfSettings) bool {
	return (p.Pwm == a.Pwm) && (p.Pwmr == a.Pwmr) && (p.Pwmc == a.Pwmc)
}

func (p *RfSettings) EqualNear(a RfSettings) bool {
	return ((p.Pwm == a.Pwm) || (p.Pwm == a.Pwm-1) || (p.Pwm == a.Pwm+1)) && (p.Pwmr == a.Pwmr) && (p.Pwmc == a.Pwmc)
}

type RfPulseSettings struct {
	On  time.Duration
	Off time.Duration
}

func (p *RfPulseSettings) Equal(a RfPulseSettings) bool {
	return (p.On == a.On) && (p.Off == a.Off)
}

/*
FROM:
RNG1: (32bit)
This register is used to define the range for the corresponding channel. In PWM mode
evenly distributed pulses are sent within a period of length defined by this register.

DAT1: (32bit)
In PWM mode data is sent by pulse width modulation: the value of this register defines
the number of pulses which is sent within the period defined by PWM_RNGi.
*/

func (p *RfSettings) GetTiming() RfPulseSettings {
	result := RfPulseSettings{}
	blockTime := PWMCLOCKNS * float64(p.Pwmc)
	result.On = time.Duration(blockTime*float64(p.Pwm)) * time.Nanosecond
	result.Off = time.Duration(blockTime*float64(p.Pwmr-p.Pwm)) * time.Nanosecond
	return result
}

/*
example:
pwmc 192
pwmr 2000
pwm 100
------

blockTime=52.0833333333*192 =9999.999999993599 ~ 10000ns= 10us  is after dividing freq.

on=100*10us = 1000us=1ms
off=10us * (2000-100)= 19ms

Another direction...
tOn=1ms = 1000000 ns
tOff=19ms 19000000 ns

in clocks
period clocks = 20000000/52.0833333333  =384000 clocks
on clocks  = 1000000/52.0833333333 = 19200 clocks
off clocks = 19000000/52.0833333333 = 364800 clocks

so why not pwmc=2 (actual 1),   pwm=19200,  and pwmr=384000

maximum of pwmc is MAXPWMC
and maximum of pwmr is  4096
SO... must divide 384000/4096=93.75  .... ceiling up to 94



pwmc=94
pwm=19200/94=204.255319... floor??  204
pwmr=384000/94= floored 4085
TODO split into common dividers...
Largest common divider is  192000  :(  because

SO pwmc must be at least 94  AND it have limitation of 4095

Now it have to optimized... what is the most accurate.

Naive solution: use



*/

/*
Get settings optimized for fast use (fatpuppy)
*/
func (p *RfPulseSettings) GetFastSettings() RfSettings {
	if (p.On.Nanoseconds() == 0) || (p.Off.Nanoseconds() == 0) {
		return RfSettings{} //do not give anything stupid
	}

	result := RfSettings{Pwmc: 2} //As fast as possible  pwmc=1 or pwmc=2 ???  pwmc=1 is problem???
	result.Pwmr = uint32(float64(p.On.Nanoseconds()+p.Off.Nanoseconds())/PWMCLOCKNS) + 1
	result.Pwm = uint32(float64(p.On.Nanoseconds())/PWMCLOCKNS) + 1
	return result
}

func (p *RfPulseSettings) GetSettings() (RfSettings, error) {
	return p.GetNaiveSettings()
}

/*
func (p *RfPulseSettings) IsNear(ref RfPulseSettings, onDeviationPercent float32, offDeviationPercent float32) bool {
	math.Abs(p.Off.Nanoseconds() - ref.Off.Nanoseconds())
	math.Abs
	deltaOnNs := p.On.Nanoseconds() - ref.On.Nanoseconds()
}
*/
func (p *RfPulseSettings) DeviationPercent(ref RfPulseSettings) (float64, float64) {
	dOff := 100 * math.Abs(float64(p.Off.Nanoseconds()-ref.Off.Nanoseconds())/float64(p.Off.Nanoseconds()))
	dOn := 100 * math.Abs(float64(p.On.Nanoseconds()-ref.On.Nanoseconds())/float64(p.On.Nanoseconds()))

	return math.Floor(dOff), math.Floor(dOn)
}

/*
Optimize by pulse accuracy
*/
/*
func (p *RfPulseSettings) GetPulseOptimalSettings() RfSettings {
	result := RfSettings{}
}
*/

//Naive solution, rouding errors.... take smallest common deteminator
func (p *RfPulseSettings) GetNaiveSettings() (RfSettings, error) {
	if (p.On.Nanoseconds() == 0) || (p.Off.Nanoseconds() == 0) {
		return RfSettings{}, nil //do not give anything stupid
	}

	result := RfSettings{}
	//getting maximum
	clocksPerPeriod := uint32(math.Ceil(float64(p.On.Nanoseconds()+p.Off.Nanoseconds()) / PWMCLOCKNS)) //How many times PWM clock will clock on period
	//Pick minimal divider = largest possible frequency

	result.Pwmc = uint32(math.Ceil(float64(clocksPerPeriod)/float64(MAXPWMR))) + 1 //1 added for technical reasons
	if result.Pwmc < MINPWMC {
		result.Pwmc = MINPWMC
	}
	result.Pwmr = uint32(math.Floor(float64(clocksPerPeriod) / float64(result.Pwmc)))
	//Now calculate how many points are required for period
	/*
		blockTime := int64(PWMCLOCKNS * float64(result.Pwmc)) //This tells how long one block takes
		result.Pwm = uint32(p.On.Nanoseconds() / blockTime)
	*/
	//This tells how long one block takes
	result.Pwm = uint32(math.Floor(float64(p.On.Nanoseconds())/(PWMCLOCKNS*float64(result.Pwmc)))) + 1

	//Checkup
	if result.Pwmr <= result.Pwm {
		return result, fmt.Errorf("Cant do requested on/off  %v/%v us pulse", float64(p.On.Nanoseconds())/1000.0, float64(p.Off.Nanoseconds())/1000.0)
	}

	if 4095 < result.Pwmc {
		return result, fmt.Errorf("Cant do requested on/off  %v/%v us pulse pwmc can not be divided more", float64(p.On.Nanoseconds())/1000.0, float64(p.Off.Nanoseconds())/1000.0)
	}

	return result, nil
}

func (p *RfSettings) SetFromSettings(newSettings RfPulseSettings) error { //Just helper function
	s, err := newSettings.GetSettings()
	if err != nil {
		return err
	}
	p.Pwm = s.Pwm
	p.Pwmc = s.Pwmc
	p.Pwmr = s.Pwmr
	return nil
}

const (
	HWPWM0PIN = 18
)

func (p *RfSettings) IsOffline() bool {
	return (p.Pwm == 0) && (p.Pwmc == 0) && (p.Pwmr == 0)
}

func (p *RaspiHw) SetToHwPWM0(rf *RfSettings) {
	p.PinMode(HWPWM0PIN, ALT5)             //ALT5 function for 18 is PWM0
	p.PwmSetMode(true, true, false, false) // PwmSetMode(en0 bool, ms0 bool, en1 bool, ms1 bool)   enable and set to mark-space mode
	p.PwmSetClock(uint32(rf.Pwmc))
	p.Pwm0SetRange(uint32(rf.Pwmr))
	p.Pwm0Set(uint32(rf.Pwm))
}

func (p *RaspiHw) SetPWM0Hi() {
	p.PwmSetMode(false, false, false, false)
	p.PinMode(HWPWM0PIN, ALToutput)
	p.PinSet(HWPWM0PIN) //TODO HI-Z?  Better drive...
}

func (p *RaspiHw) SetPWM0Lo() {
	p.PwmSetMode(false, false, false, false)
	p.PinMode(HWPWM0PIN, ALToutput)
	p.PinClear(HWPWM0PIN) //TODO HI-Z?  Better drive...
}
