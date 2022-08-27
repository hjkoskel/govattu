govattu
=======

![govattu](govattu.png)

Golang library (or just simple wrapper) for accessing /dev/mem features on raspberry pi

based on wiringPi (http://wiringpi.com/)  and
http://www.airspayce.com/mikem/bcm2835/
Butchered go-rpio library and mutilated to this library with added features
https://github.com/stianeikeland/go-rpio
Feel free to merge hardwarePWM feature to go-rpio if my API design sucks.

## Features ##
- GPIO  input/output
- GPIO events  rise/fall/level/asyncrise/asyncfall
  -Nice to use with command buttons, can do without aggressive polling
- HardwarePWM
  - only PWM0 tested

## Features to do ##
- Raspberry jams with /dev/gpiomem when writing to mempwm[PWMMEM_RNG1] Before solving problem, run this  as root  (sudo)
- Multiple pin reads and writes. For parrallel interfacing LCDs etc...

## Design principles ##
- No c code, no external dependencies
- This library contains only tricks that are not implemented with drivers like UART or I2C or SPI
- Only low level functionalities, no lcd functions or software pwm. They are implemented on higher levels
- API is going to freeze, but not yet

## Pin numbering
Govattu uses so called BCM numbering. BCM and GPIO numbers go with same way

## How to use ##

```go
import "github.com/hjkoskel/govattu"
```
Initialize
```go
hw, err := govattu.Open()
```
For creating unit tests or mocups there is inteface called *Vattu* and *DoNothingPi* example implementation



Basic gpio functions are
```go
hw.PinClear(pin uint8) //for output 0
hw.PinSet(pin uint8) //For output 1
hw.ReadPinLevel(pin uint8) bool  //For input
hw.PullMode(pin uint8, pull PinPull) //Set pin pullup setting  govattu.PULLoff  or govattu.PULLdown or govattu.PULLup
```

Setting up hardwarePWM.
If calculating divisors and ranges is hard. Please check simple example project
https://github.com/hjkoskel/pipwm

```go
hw.PinMode(18, govattu.ALT5) //ALT5 function for 18 is PWM0
hw.PwmSetMode(true, true, false, false) // PwmSetMode(en0 bool, ms0 bool, en1 bool, ms1 bool)   enable and set to mark-space mode for pwm0 and pwm1
hw.PwmSetClock(divisor uint32)  //Set clock divisor
hw.Pwm0SetRange(ra uint32) //SET RANGE
hw.Pwm0Set(r uint32) //Set pwm
```

This library also have function for setting events flags. Very usefull if code monitors button presses. It is handy to detect fall event and later check "did events happend"

```go
hw.ZeroPinEventDetectMask() //At initialization
/*
Possible flags (bitwise or together)
PINE_AFALL
PINE_ARISE
PINE_LOW
PINE_HIGH
PINE_FALL
PINE_RISE

*/
hw.SetPinEventDetectMask(buttonPinNumber, govattu.PINE_FALL|govattu.PINE_RISE) //Set event detection flags for specific pin

//And when polling detected events
if hw.ReadPinEvent(buttonPinNumber) {
  //Process event here
	hw.ResetPinEvent(buttonPinNumber) //This is needed for clearing before next readPinEvent
}
```
