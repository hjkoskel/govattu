govattu
=======
Golang library (or just simple wrapper) for accessing /dev/mem features on raspberry pi

based on wiringPi (http://wiringpi.com/)  and
http://www.airspayce.com/mikem/bcm2835/
Butchered go-rpio library and mutilated to this library with added features
https://github.com/stianeikeland/go-rpio
Feel free to merge hardwarePWM feature to go-rpio if my API design sucks.

## Features ##
-GPIO  input/output
-GPIO events  rise/fall/level/asyncrise/asyncfall
  -Nice to use with command buttons, can do without aggressive polling
-HardwarePWM
  -only PWM0 tested

## Features to do ##
-Raspberry jams with /dev/gpiomem when writing to mempwm[PWMMEM_RNG1] Before solving problem, run this  as root  (sudo)
-Multiple pin reads and writes. For parrallel interfacing LCDs etc...

## Design principles ##
- No c code, no external dependencies
- This library contains only tricks that are not implemented with drivers like UART or I2C or SPI
- Only low level functionalities, no lcd functions or software pwm. They are implemented on higher levels
- API is going to freeze, but not yet

## How to use ##

```go
import "github.com/hjkoskel/govattu"
```
Initialize
```go
hw, err := govattu.Open()
if err != nil {
  return err
}
defer hw.Close()
```
hw given by open is needed only when closing connection. Later there will be some other functionalities

Then select pin function input output or special functions to all pins you need (on pin 18 ALT5 is PWM0)
```go
govattu.PinMode(23, govattu.ALToutput)
govattu.PinMode(24, govattu.ALTinput)
govattu.PinMode(18, govattu.ALT5) //ALT5 function for 18 is PWM0
```
Basic gpio functions are
```go
govattu.PinClear(pin uint8) //for output 0
govattu.PinSet(pin uint8) //For output 1
govattu.ReadPinLevel(pin uint8) bool  //For input
govattu.PullMode(pin uint8, pull PinPull) //Set pin pullup setting  govattu.PULLoff  or govattu.PULLdown or govattu.PULLup
```

Setting up hardwarePWM.
If calculating divisors and ranges is hard. Please check simple example project
https://github.com/hjkoskel/pipwm

```go
govattu.PinMode(18, govattu.ALT5) //ALT5 function for 18 is PWM0
govattu.PwmSetMode(true, true, false, false) // PwmSetMode(en0 bool, ms0 bool, en1 bool, ms1 bool)   enable and set to mark-space mode for pwm0 and pwm1
govattu.PwmSetClock(divisor uint32)  //Set clock divisor
govattu.Pwm0SetRange(ra uint32) //SET RANGE
govattu.Pwm0Set(r uint32) //Set pwm
```

This library also have function for setting events flags. Very usefull if code monitors button presses. It is handy to detect fall event and later check "did events happend"

```go
govattu.ZeroPinEventDetectMask() //At initialization
/*
Possible flags (bitwise or together)
PINE_AFALL
PINE_ARISE
PINE_LOW
PINE_HIGH
PINE_FALL
PINE_RISE

*/
govattu.SetPinEventDetectMask(buttonPinNumber, govattu.PINE_FALL|govattu.PINE_RISE) //Set event detection flags for specific pin

//And when polling detected events
if govattu.ReadPinEvent(buttonPinNumber) {
  //Process event here
	govattu.ResetPinEvent(buttonPinNumber) //This is needed for clearing before next readPinEvent
}
```
