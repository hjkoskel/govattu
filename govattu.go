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

const (
	bcm2835Base uint32 = 0x20000000 //Old base, first quess
	BCMPASSWORD uint32 = 0x5A000000
)

//Memory map sizes
const (
	CLKLEN  = 0xA8
	GPIOLEN = 0xB4
	PWMLEN  = 0x28
)

const (
	BASEADDRCLK  uint32 = 0x00101000
	BASEADDRGPIO uint32 = 0x00200000
	BASEADDRPWM  uint32 = 0x0020C000
)

// 0x7E00 0000 is base address on datasheet
const (
	GPIOMEMOFF_MODE         uint32 = 0
	GPIOMEMOFF_SET          uint32 = 7
	GPIOMEMOFF_CLR          uint32 = 10
	GPIOMEMOFF_LEVEL        uint32 = 13
	GPIOMEMOFF_EVENTDET     uint32 = 16
	GPIOMEMOFF_RISEVENTDET  uint32 = 19
	GPIOMEMOFF_FALLEVENTDET uint32 = 22
	GPIOMEMOFF_HIDET        uint32 = 25
	GPIOMEMOFF_LODET        uint32 = 28
	GPIOMEMOFF_ASYNCRISEDET uint32 = 31
	GPIOMEMOFF_ASYNCFALLDET uint32 = 34
	GPIOMEMOFF_PULL         uint32 = 37
	GPIOMEMOFF_PULLCLK      uint32 = 38
)

//Clock memory area register names from datasheet
const (
	CM_GP0CTL uint32 = 0
	CM_GP0DIV uint32 = 1
	CM_GP1CTL uint32 = 2
	CM_GP1DIV uint32 = 3
	CM_GP2CTL uint32 = 4
	CM_GP2DIV uint32 = 5
)

//From GPIO base

var memlock sync.Mutex
var (
	memGpio  []uint32
	mem8Gpio []uint8
	mempwm   []uint32
	mem8pwm  []uint8
	memclk   []uint32
	mem8clk  []uint8
)

//Just empty struct. Might require some stored state later
type RaspiHw struct {
}

//	000 = GPIO Pin X is an input
//	001 = GPIO Pin X is an output
//	010 = GPIO Pin X takes alternate function 5
//	011 = GPIO Pin X takes alternate function 4
//	100 = GPIO Pin X takes alternate function 0
//	101 = GPIO Pin X takes alternate function 1
//	110 = GPIO Pin X takes alternate function 2
//	111 = GPIO Pin X takes alternate function 3

type AltSetting byte

const (
	ALTinput AltSetting = iota
	ALToutput
	ALT5
	ALT4
	ALT0
	ALT1
	ALT2
	ALT3
)

type PinConfig struct {
	Alt byte
}

func Open() (RaspiHw, error) {
	result := RaspiHw{}
	base := getBCMBase()

	//unix.Open allows to experiment with more flags and options than os.OpenFile

	fd, err := unix.Open("/dev/mem", unix.O_RDWR|unix.O_SYNC|unix.O_CLOEXEC, 0)
	if err != nil {
		fmt.Printf(" /dev/mem error is %v  so trying gpiomem\n", err)
		fmt.Printf("TODO PROBLEM TO BE SOLVED!!\nRaspberry jams with /dev/gpiomem when writing to mempwm[PWMMEM_RNG1]\nBefore solving problem, run this  as root  (sudo)\n")
		os.Exit(-1)
		/*
			fd, err = unix.Open("/dev/gpiomem", unix.O_RDWR|unix.O_SYNC|unix.O_CLOEXEC, 0)
			fmt.Printf("Using /dev/gpiomem and base=0\n")
			base = 0
		*/
	}

	if err != nil {
		return RaspiHw{}, err
	}
	// FD can be closed after memory mapping
	defer unix.Close(fd)

	memlock.Lock()
	defer memlock.Unlock()

	//Separate memory banks
	mem8Gpio, err = syscall.Mmap(
		fd,
		int64(base+BASEADDRGPIO),
		GPIOLEN, //TODO enough?
		syscall.PROT_READ|syscall.PROT_WRITE|syscall.PROT_EXEC,
		syscall.MAP_SHARED|syscall.MAP_LOCKED)
	if err != nil {
		return result, err
	}

	mem8clk, err = syscall.Mmap(
		fd,
		int64(base+BASEADDRCLK),
		CLKLEN, //TODO enough?
		syscall.PROT_READ|syscall.PROT_WRITE|syscall.PROT_EXEC,
		syscall.MAP_SHARED|syscall.MAP_LOCKED)
	if err != nil {
		return result, err
	}

	mem8pwm, err = syscall.Mmap(
		fd,
		int64(base+BASEADDRPWM),
		PWMLEN, //TODO enough?
		syscall.PROT_READ|syscall.PROT_WRITE|syscall.PROT_EXEC,
		syscall.MAP_SHARED|syscall.MAP_LOCKED)
	if err != nil {
		return result, err
	}

	// Convert mapped byte memory to unsafe []uint32 pointer, adjust length as needed
	headerGpio := *(*reflect.SliceHeader)(unsafe.Pointer(&mem8Gpio))
	headerGpio.Len /= (32 / 8) // (32 bit = 4 bytes)
	headerGpio.Cap /= (32 / 8)
	memGpio = *(*[]uint32)(unsafe.Pointer(&headerGpio))

	headerClk := *(*reflect.SliceHeader)(unsafe.Pointer(&mem8clk))
	headerClk.Len /= (32 / 8) // (32 bit = 4 bytes)
	headerClk.Cap /= (32 / 8)
	memclk = *(*[]uint32)(unsafe.Pointer(&headerClk))

	headerPwm := *(*reflect.SliceHeader)(unsafe.Pointer(&mem8pwm))
	headerPwm.Len /= (32 / 8) // (32 bit = 4 bytes)
	headerPwm.Cap /= (32 / 8)
	mempwm = *(*[]uint32)(unsafe.Pointer(&headerPwm))
	return result, err
}

const (
	PWMMEM_CTL  uint32 = 0
	PWMMEM_STA  uint32 = 1
	PWMMEM_DMAC uint32 = 2
	PWMMEM_RNG1 uint32 = 4
	PWMMEM_DAT1 uint32 = 5
	PWMMEM_FIF1 uint32 = 6
	PWMMEM_RNG2 uint32 = 8
	PWMMEM_DAT2 uint32 = 9
)

//these come from wiringpi
const (
	PWMCLK_CNTL uint32 = 40
	PWMCLK_DIV  uint32 = 41
)

//Flages to PWM regs
const (
	PWM0_ENABLE  uint32 = 0x0001
	PWM1_ENABLE  uint32 = 0x0100
	PWM0_MS_MODE uint32 = 0x0080
	PWM1_MS_MODE uint32 = 0x8000
)

/*
enable and disable.
Set to mark-space mode
*/
func PwmSetMode(en0 bool, ms0 bool, en1 bool, ms1 bool) {
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
	mempwm[PWMMEM_CTL] = w
}

func Pwm0SetRange(ra uint32) {
	memlock.Lock()
	defer memlock.Unlock()
	mempwm[PWMMEM_RNG1] = ra
	time.Sleep(time.Microsecond * 10)
}

func Pwm1SetRange(ra uint32) {
	memlock.Lock()
	defer memlock.Unlock()
	mempwm[PWMMEM_RNG2] = ra
	time.Sleep(time.Microsecond * 10)
}

func Pwm0Set(r uint32) {
	memlock.Lock()
	defer memlock.Unlock()
	mempwm[PWMMEM_DAT1] = r
}

func Pwm1Set(r uint32) {
	memlock.Lock()
	defer memlock.Unlock()
	mempwm[PWMMEM_DAT2] = r
}

/*
TODO not quite clear why
PWMCLK_CNTL uint32 = 40
PWMCLK_DIV  uint32 = 41

https://groups.google.com/forum/#!topic/bcm2835/_foWTDSwF8k
this code relies on the description given in
http://www.scribd.com/doc/127599939/BCM2835-Audio-clocks#scribd

*/
func PwmSetClock(divisor uint32) {
	memlock.Lock()
	defer memlock.Unlock()
	pwmcontrolPreserved := mempwm[PWMMEM_CTL] // preserve PWMMEM_CTL
	mempwm[PWMMEM_CTL] = 0

	// Stop PWM clock before changing divisor. The delay after this does need to
	// this big (95uS occasionally fails, 100uS OK), it's almost as though the BUSY
	// flag is not working properly in balanced mode. Without the delay when DIV is
	// adjusted the clock sometimes switches to very slow, once slow further DIV
	// adjustments do nothing and it's difficult to get out of this mode.
	memclk[PWMCLK_CNTL] = BCMPASSWORD | 0x01 // Stop PWM Clock, Set only clock source bit to oscillator

	time.Sleep(time.Microsecond * 110) // prevents clock going sloooow

	//while((*(clk + PWMCLK_CNTL) & 0x80) != 0) // Wait for clock to be !BUSY
	for memclk[PWMCLK_CNTL]&0x80 != 0 {
	}
	time.Sleep(time.Microsecond)

	memclk[PWMCLK_DIV] = BCMPASSWORD | (divisor << 12)
	memclk[PWMCLK_CNTL] = BCMPASSWORD | 0x11 // Start PWM clock, clock source bit and  enable bit active
	mempwm[PWMMEM_CTL] = pwmcontrolPreserved // restore PWMMEM_CTL
}

// Close unmaps GPIO memory
func (p *RaspiHw) Close() error {
	memlock.Lock()
	defer memlock.Unlock()
	syscall.Munmap(mem8Gpio)
	syscall.Munmap(mem8clk)
	return syscall.Munmap(mem8pwm)
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
	fmt.Printf("Out base is %v\n", out)
	return out
}

func PinClear(pin uint8) {
	memlock.Lock()
	defer memlock.Unlock()
	memGpio[uint32(pin/32)+GPIOMEMOFF_CLR] = 1 << (pin & 31)
}

func PinSet(pin uint8) {
	memlock.Lock()
	defer memlock.Unlock()
	memGpio[uint32(pin/32)+GPIOMEMOFF_SET] = 1 << (pin & 31)
}

func ReadPinLevel(pin uint8) bool {
	return ((memGpio[uint32(pin/32)+GPIOMEMOFF_LEVEL] & (1 << pin)) != 0)
}
func ReadAllPinLevels() uint64 {
	return uint64(memGpio[GPIOMEMOFF_LEVEL]) | (uint64(memGpio[GPIOMEMOFF_LEVEL+1]) << 32) //TODO endiand does not matter, just grab all :)
}

func ReadPinEvent(pin uint8) bool {
	return ((memGpio[uint32(pin/32)+GPIOMEMOFF_EVENTDET] & (1 << pin)) != 0)
}
func ReadAllPinEvents() uint64 {
	return uint64(memGpio[GPIOMEMOFF_EVENTDET]) | (uint64(memGpio[GPIOMEMOFF_EVENTDET+1]) << 32) //TODO endiand does not matter, just grab all :)
}

func ResetPinEvent(pin uint8) {
	memlock.Lock()
	defer memlock.Unlock()
	memGpio[uint32(pin/32)+GPIOMEMOFF_EVENTDET] = 1 << (pin & 31)
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

func ZeroPinEventDetectMask() {
	memlock.Lock()
	defer memlock.Unlock()
	//TODO LOOP RANGE
	memGpio[GPIOMEMOFF_EVENTDET] = 0
	memGpio[GPIOMEMOFF_RISEVENTDET] = 0
	memGpio[GPIOMEMOFF_FALLEVENTDET] = 0
	memGpio[GPIOMEMOFF_HIDET] = 0
	memGpio[GPIOMEMOFF_LODET] = 0
	memGpio[GPIOMEMOFF_ASYNCRISEDET] = 0
	memGpio[GPIOMEMOFF_ASYNCFALLDET] = 0

	memGpio[GPIOMEMOFF_EVENTDET+1] = 0
	memGpio[GPIOMEMOFF_RISEVENTDET+1] = 0
	memGpio[GPIOMEMOFF_FALLEVENTDET+1] = 0
	memGpio[GPIOMEMOFF_HIDET+1] = 0
	memGpio[GPIOMEMOFF_LODET+1] = 0
	memGpio[GPIOMEMOFF_ASYNCRISEDET+1] = 0
	memGpio[GPIOMEMOFF_ASYNCFALLDET+1] = 0

}

func SetPinEventDetectMask(pin uint8, pine PinEventMask) {
	memlock.Lock()
	defer memlock.Unlock()
	//Dummy solution... just clear all and then set
	offset := uint32(pin / 32)
	mask := uint32(1) << uint32(pin&31)

	if 0 < pine&PINE_AFALL {
		memGpio[GPIOMEMOFF_ASYNCFALLDET+offset] |= mask
	}
	if 0 < pine&PINE_ARISE {
		memGpio[GPIOMEMOFF_ASYNCRISEDET] |= mask
	}
	if 0 < pine&PINE_LOW {
		memGpio[GPIOMEMOFF_LODET] |= mask
	}
	if 0 < pine&PINE_HIGH {
		memGpio[GPIOMEMOFF_HIDET] |= mask
	}
	if 0 < pine&PINE_FALL {
		memGpio[GPIOMEMOFF_FALLEVENTDET] |= mask
	}
	if 0 < pine&PINE_RISE {
		memGpio[GPIOMEMOFF_RISEVENTDET] |= mask
	}
}

type PinPull byte

//Raspberry have pull up and downs
const (
	PULLoff PinPull = iota
	PULLdown
	PULLup
)

func PullMode(pin uint8, pull PinPull) {
	// Pull up/down/off register has offset 38 / 39, pull is 37
	memlock.Lock()
	defer memlock.Unlock()

	switch pull {
	case PULLdown, PULLup:
		memGpio[GPIOMEMOFF_PULL] = memGpio[GPIOMEMOFF_PULL]&^3 | uint32(pull)
	case PULLoff:
		memGpio[GPIOMEMOFF_PULL] = memGpio[GPIOMEMOFF_PULL] &^ 3
	}
	// Wait for value to clock in, this is ugly, sorry :(
	time.Sleep(time.Microsecond)
	memGpio[uint32(pin/32)+GPIOMEMOFF_PULLCLK] = 1 << (pin % 32)
	// Wait for value to clock in
	time.Sleep(time.Microsecond)
	memGpio[GPIOMEMOFF_PULLCLK] = memGpio[GPIOMEMOFF_PULLCLK] &^ 3
	memGpio[GPIOMEMOFF_PULLCLK] = 0
}

func PinMode(pin uint8, alt AltSetting) {
	fsel := uint32(pin)/10 + GPIOMEMOFF_MODE
	shift := (uint8(pin) % 10) * 3
	memlock.Lock()
	defer memlock.Unlock()
	memGpio[fsel] = (memGpio[fsel] &^ (uint32(0x7) << shift)) | uint32(alt)<<shift
}
