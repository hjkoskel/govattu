package govattu

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
