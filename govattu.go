/*
GoVattu is hardware wrapper library for raspberry.

features
-GPIO  input/output
-GPIO events  rise/fall/level/asyncrise/asyncfall
	-Nice to use with command buttons, can do without aggressive polling
-HardwarePWM
	only PWM0 tested

pro-tip: https://groups.google.com/forum/#!topic/bcm2835/_foWTDSwF8k

does also some GPIO

based on wiringPi (http://wiringpi.com/)  and
http://www.airspayce.com/mikem/bcm2835/

Butchered go-rpio library and mutilated to this library with added features
https://github.com/stianeikeland/go-rpio

Feel free to merge hardwarePWM feature to go-rpio if my API design sucks


Features to do:
- Raspberry jams with /dev/gpiomem when writing to mempwm[PWMMEM_RNG1]\nBefore solving problem, run this  as root  (sudo)
- Multiple pin reads and writes with one lock and register read. For parrallel interfacing

Design principle:
0) No c code, no external dependencies
1) This library contains only tricks that are not implemented with drivers like UART or I2C or SPI
2) Only low level functionalities, no HD44780 parrallel functions or software pwm
	They are implemented on higher levels
3) Library API is going to freeze at some point, but not at this early stage
USE THIS LIBRARY WITH OUR OWN RISK

*/

package govattu

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"reflect"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

//From GPIO base
var memlock sync.Mutex //There is only one hardware, make global

type PinConfig struct {
	Alt byte
}

//Vattu is interface for interfacing real hardware or fake
type Vattu interface {
	PwmSetMode(en0 bool, ms0 bool, en1 bool, ms1 bool)
	Pwm0SetRange(ra uint32)
	Pwm1SetRange(ra uint32)
	Pwm0Set(r uint32)
	Pwm1Set(r uint32)

	SetPWM0Hi()
	SetPWM0Lo()
	SetToHwPWM0(rf *RfSettings)

	PwmSetClock(divisor uint32)
	Close() error

	PinClear(pin uint8)
	PinSet(pin uint8)
	PinMode(pin uint8, alt AltSetting)
	ReadPinLevel(pin uint8) bool
	ReadAllPinLevels() uint64
	ReadPinEvent(pin uint8) bool
	ReadAllPinEvents() uint64
	ResetPinEvent(pin uint8)
	ZeroPinEventDetectMask()
	SetPinEventDetectMask(pin uint8, pine PinEventMask)
	PullMode(pin uint8, pull PinPull)
}

//Just empty struct. Might require some stored state later
type RaspiHw struct {
	memGpio  []uint32
	mem8Gpio []uint8
	mempwm   []uint32
	mem8pwm  []uint8
	memclk   []uint32
	mem8clk  []uint8
}

func Open() (Vattu, error) {
	result := RaspiHw{}
	base := getBCMBase()

	//unix.Open allows to experiment with more flags and options than os.OpenFile
	fd, err := unix.Open("/dev/mem", unix.O_RDWR|unix.O_SYNC|unix.O_CLOEXEC, 0)
	//fd, err := unix.Open("/dev/gpiomem", unix.O_RDWR|unix.O_SYNC|unix.O_CLOEXEC, 0)
	if err != nil {
		//return result,fmt.Errorf(" /dev/mem error is %v  so trying gpiomem. TODO PROBLEM TO BE SOLVED!!\nRaspberry jams with /dev/gpiomem when writing to mempwm[PWMMEM_RNG1]\nBefore solving problem, run this  as root  (sudo)", err)
		fmt.Printf(" /dev/mem error is %v  so trying gpiomem.", err.Error())
		fd, err = unix.Open("/dev/gpiomem", unix.O_RDWR|unix.O_SYNC|unix.O_CLOEXEC, 0)
		fmt.Printf("Using /dev/gpiomem and base=0\n")
		base = 0

	}

	if err != nil {
		return Vattu(&RaspiHw{}), fmt.Errorf("Error while opening memory err=%v", err.Error())
	}

	// FD can be closed after memory mapping
	defer unix.Close(fd)

	memlock.Lock()
	defer memlock.Unlock()

	//Separate memory banks
	result.mem8Gpio, err = syscall.Mmap(
		fd,
		int64(base+BASEADDRGPIO),
		GPIOLEN, //TODO enough?
		syscall.PROT_READ|syscall.PROT_WRITE|syscall.PROT_EXEC,
		syscall.MAP_SHARED|syscall.MAP_LOCKED)
	if err != nil {
		return Vattu(&result), fmt.Errorf("syscall.Mmap mem8Gpio fail on init %v", err.Error())
	}

	result.mem8clk, err = syscall.Mmap(
		fd,
		int64(base+BASEADDRCLK),
		CLKLEN, //TODO enough?
		syscall.PROT_READ|syscall.PROT_WRITE|syscall.PROT_EXEC,
		syscall.MAP_SHARED|syscall.MAP_LOCKED)
	if err != nil {
		return Vattu(&result), fmt.Errorf("syscall.Mmap mem8clk fail on init %v", err.Error())
	}

	result.mem8pwm, err = syscall.Mmap(
		fd,
		int64(base+BASEADDRPWM),
		PWMLEN, //TODO enough?
		syscall.PROT_READ|syscall.PROT_WRITE|syscall.PROT_EXEC,
		syscall.MAP_SHARED|syscall.MAP_LOCKED)
	if err != nil {
		return Vattu(&result), fmt.Errorf("syscall.Mmap mem8pwm fail on init %v", err.Error())
	}

	// Convert mapped byte memory to unsafe []uint32 pointer, adjust length as needed
	headerGpio := *(*reflect.SliceHeader)(unsafe.Pointer(&result.mem8Gpio))
	headerGpio.Len /= (32 / 8) // (32 bit = 4 bytes)
	headerGpio.Cap /= (32 / 8)
	result.memGpio = *(*[]uint32)(unsafe.Pointer(&headerGpio))

	headerClk := *(*reflect.SliceHeader)(unsafe.Pointer(&result.mem8clk))
	headerClk.Len /= (32 / 8) // (32 bit = 4 bytes)
	headerClk.Cap /= (32 / 8)
	result.memclk = *(*[]uint32)(unsafe.Pointer(&headerClk))

	headerPwm := *(*reflect.SliceHeader)(unsafe.Pointer(&result.mem8pwm))
	headerPwm.Len /= (32 / 8) // (32 bit = 4 bytes)
	headerPwm.Cap /= (32 / 8)
	result.mempwm = *(*[]uint32)(unsafe.Pointer(&headerPwm))
	return Vattu(&result), nil
}

/*
enable and disable.
Set to mark-space mode
*/
func (p *RaspiHw) PwmSetMode(en0 bool, ms0 bool, en1 bool, ms1 bool) {
	w := uint32(0)
	if ms0 {
		w |= PWM0_MS_MODE
	}
	if ms1 {
		w |= PWM1_MS_MODE
	}
	if en0 {
		w |= PWM0_ENABLE
	}
	if en1 {
		w |= PWM1_ENABLE
	}

	memlock.Lock()
	defer memlock.Unlock()
	p.mempwm[PWMMEM_CTL] = w
}

func (p *RaspiHw) Pwm0SetRange(ra uint32) {
	memlock.Lock()
	defer memlock.Unlock()
	p.mempwm[PWMMEM_RNG1] = ra
	time.Sleep(time.Microsecond * 10)
}

func (p *RaspiHw) Pwm1SetRange(ra uint32) {
	memlock.Lock()
	defer memlock.Unlock()
	p.mempwm[PWMMEM_RNG2] = ra
	time.Sleep(time.Microsecond * 10)
}

func (p *RaspiHw) Pwm0Set(r uint32) {
	memlock.Lock()
	defer memlock.Unlock()
	p.mempwm[PWMMEM_DAT1] = r
}

func (p *RaspiHw) Pwm1Set(r uint32) {
	memlock.Lock()
	defer memlock.Unlock()
	p.mempwm[PWMMEM_DAT2] = r
}

/*
TODO not quite clear why
PWMCLK_CNTL uint32 = 40
PWMCLK_DIV  uint32 = 41

https://groups.google.com/forum/#!topic/bcm2835/_foWTDSwF8k
this code relies on the description given in
http://www.scribd.com/doc/127599939/BCM2835-Audio-clocks#scribd
TODO: Check fractional part of divisor

*/
func (p *RaspiHw) PwmSetClock(divisor uint32) {
	memlock.Lock()
	defer memlock.Unlock()
	pwmcontrolPreserved := p.mempwm[PWMMEM_CTL] // preserve PWMMEM_CTL
	p.mempwm[PWMMEM_CTL] = 0

	// Stop PWM clock before changing divisor. The delay after this does need to
	// this big (95uS occasionally fails, 100uS OK), it's almost as though the BUSY
	// flag is not working properly in balanced mode. Without the delay when DIV is
	// adjusted the clock sometimes switches to very slow, once slow further DIV
	// adjustments do nothing and it's difficult to get out of this mode.
	p.memclk[PWMCLK_CNTL] = BCMPASSWORD | 0x01 // Stop PWM Clock, Set only clock source bit to oscillator

	time.Sleep(time.Microsecond * 110) // prevents clock going sloooow

	//while((*(clk + PWMCLK_CNTL) & 0x80) != 0) // Wait for clock to be !BUSY
	for p.memclk[PWMCLK_CNTL]&0x80 != 0 {
	}
	time.Sleep(time.Microsecond)

	p.memclk[PWMCLK_DIV] = BCMPASSWORD | (divisor << 12) //TODO fractional part of divisor???
	p.memclk[PWMCLK_CNTL] = BCMPASSWORD | 0x11           // Start PWM clock, clock source bit and  enable bit active
	p.mempwm[PWMMEM_CTL] = pwmcontrolPreserved           // restore PWMMEM_CTL
}

// Close unmaps GPIO memory
func (p *RaspiHw) Close() error {
	memlock.Lock()
	defer memlock.Unlock()
	syscall.Munmap(p.mem8Gpio)
	syscall.Munmap(p.mem8clk)
	return syscall.Munmap(p.mem8pwm)
}

// Read /proc/device-tree/soc/ranges and determine the base address.
// Use the default Raspberry Pi 1 base address if this fails.
// Use base instead of GPIO base because clock is before gpio
func getBCMBase() (base uint32) {
	base = bcm2835Base
	ranges, err := os.Open("/proc/device-tree/soc/ranges")
	defer ranges.Close()
	if err != nil {
		fmt.Printf("TODO HANDLE ERROR %v\n", err)
		return
	}
	b := make([]byte, 4)
	n, err := ranges.ReadAt(b, 4)
	if n != 4 || err != nil {
		fmt.Printf("TODO HANDLE ERROR %v\n", err)
		return
	}
	buf := bytes.NewReader(b)
	var out uint32
	err = binary.Read(buf, binary.BigEndian, &out)
	if err != nil {
		fmt.Printf("TODO HANDLE ERROR %v\n", err)
		return
	}
	return out
}

func (p *RaspiHw) PinClear(pin uint8) {
	memlock.Lock()
	defer memlock.Unlock()
	p.memGpio[uint32(pin/32)+GPIOMEMOFF_CLR] = 1 << (pin & 31)
}

func (p *RaspiHw) PinSet(pin uint8) {
	memlock.Lock()
	defer memlock.Unlock()
	p.memGpio[uint32(pin/32)+GPIOMEMOFF_SET] = 1 << (pin & 31)
}

func (p *RaspiHw) ReadPinLevel(pin uint8) bool {
	memlock.Lock()
	defer memlock.Unlock()

	return ((p.memGpio[uint32(pin/32)+GPIOMEMOFF_LEVEL] & (1 << pin)) != 0)
}
func (p *RaspiHw) ReadAllPinLevels() uint64 {
	memlock.Lock()
	defer memlock.Unlock()
	return uint64(p.memGpio[GPIOMEMOFF_LEVEL]) | (uint64(p.memGpio[GPIOMEMOFF_LEVEL+1]) << 32) //TODO endiand does not matter, just grab all :)
}

func (p *RaspiHw) ReadPinEvent(pin uint8) bool {
	memlock.Lock()
	defer memlock.Unlock()
	return ((p.memGpio[uint32(pin/32)+GPIOMEMOFF_EVENTDET] & (1 << pin)) != 0)
}
func (p *RaspiHw) ReadAllPinEvents() uint64 {
	memlock.Lock()
	defer memlock.Unlock()
	return uint64(p.memGpio[GPIOMEMOFF_EVENTDET]) | (uint64(p.memGpio[GPIOMEMOFF_EVENTDET+1]) << 32) //TODO endiand does not matter, just grab all :)
}

func (p *RaspiHw) ResetPinEvent(pin uint8) {
	memlock.Lock()
	defer memlock.Unlock()
	p.memGpio[uint32(pin/32)+GPIOMEMOFF_EVENTDET] = 1 << (pin & 31)
}

//PINE,  Pin Event masks
type PinEventMask byte

const (
	PINE_AFALL PinEventMask = 1
	PINE_ARISE PinEventMask = 2
	PINE_LOW   PinEventMask = 4
	PINE_HIGH  PinEventMask = 8
	PINE_FALL  PinEventMask = 16
	PINE_RISE  PinEventMask = 32
)

func (p *RaspiHw) ZeroPinEventDetectMask() {
	memlock.Lock()
	defer memlock.Unlock()
	//TODO LOOP RANGE
	p.memGpio[GPIOMEMOFF_EVENTDET] = 0
	p.memGpio[GPIOMEMOFF_RISEVENTDET] = 0
	p.memGpio[GPIOMEMOFF_FALLEVENTDET] = 0
	p.memGpio[GPIOMEMOFF_HIDET] = 0
	p.memGpio[GPIOMEMOFF_LODET] = 0
	p.memGpio[GPIOMEMOFF_ASYNCRISEDET] = 0
	p.memGpio[GPIOMEMOFF_ASYNCFALLDET] = 0

	p.memGpio[GPIOMEMOFF_EVENTDET+1] = 0
	p.memGpio[GPIOMEMOFF_RISEVENTDET+1] = 0
	p.memGpio[GPIOMEMOFF_FALLEVENTDET+1] = 0
	p.memGpio[GPIOMEMOFF_HIDET+1] = 0
	p.memGpio[GPIOMEMOFF_LODET+1] = 0
	p.memGpio[GPIOMEMOFF_ASYNCRISEDET+1] = 0
	p.memGpio[GPIOMEMOFF_ASYNCFALLDET+1] = 0

}

func (p *RaspiHw) SetPinEventDetectMask(pin uint8, pine PinEventMask) {
	memlock.Lock()
	defer memlock.Unlock()
	//Dummy solution... just clear all and then set
	offset := uint32(pin / 32)
	mask := uint32(1) << uint32(pin&31)

	if 0 < pine&PINE_AFALL {
		p.memGpio[GPIOMEMOFF_ASYNCFALLDET+offset] |= mask
	}
	if 0 < pine&PINE_ARISE {
		p.memGpio[GPIOMEMOFF_ASYNCRISEDET] |= mask
	}
	if 0 < pine&PINE_LOW {
		p.memGpio[GPIOMEMOFF_LODET] |= mask
	}
	if 0 < pine&PINE_HIGH {
		p.memGpio[GPIOMEMOFF_HIDET] |= mask
	}
	if 0 < pine&PINE_FALL {
		p.memGpio[GPIOMEMOFF_FALLEVENTDET] |= mask
	}
	if 0 < pine&PINE_RISE {
		p.memGpio[GPIOMEMOFF_RISEVENTDET] |= mask
	}
}

type PinPull byte

//Raspberry have pull up and downs
const (
	PULLoff PinPull = iota
	PULLdown
	PULLup
)

func (p *RaspiHw) PullMode(pin uint8, pull PinPull) {
	// Pull up/down/off register has offset 38 / 39, pull is 37
	memlock.Lock()
	defer memlock.Unlock()

	switch pull {
	case PULLdown, PULLup:
		p.memGpio[GPIOMEMOFF_PULL] = p.memGpio[GPIOMEMOFF_PULL]&^3 | uint32(pull)
	case PULLoff:
		p.memGpio[GPIOMEMOFF_PULL] = p.memGpio[GPIOMEMOFF_PULL] &^ 3
	}
	// Wait for value to clock in, this is ugly, sorry :(
	time.Sleep(time.Microsecond)
	p.memGpio[uint32(pin/32)+GPIOMEMOFF_PULLCLK] = 1 << (pin % 32)
	// Wait for value to clock in
	time.Sleep(time.Microsecond)
	p.memGpio[GPIOMEMOFF_PULLCLK] = p.memGpio[GPIOMEMOFF_PULLCLK] &^ 3
	p.memGpio[GPIOMEMOFF_PULLCLK] = 0
}

func (p *RaspiHw) PinMode(pin uint8, alt AltSetting) {
	fsel := uint32(pin)/10 + GPIOMEMOFF_MODE
	shift := (uint8(pin) % 10) * 3
	memlock.Lock()
	defer memlock.Unlock()
	p.memGpio[fsel] = (p.memGpio[fsel] &^ (uint32(0x7) << shift)) | uint32(alt)<<shift
}
