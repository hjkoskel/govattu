/*
Test can be done only for function that calculate something.
Like hardware PWM.  Calulation should be "stable" when going regs->timing->regs

*/

package govattu_test

import (
	"fmt"
	"testing"

	"github.com/hjkoskel/govattu"
)

/*
func TestGpioPins(t *testing.T) {
	raspiHw, errSim := govattu.OpenAsSimulated()
	if errSim != nil {
		t.Error(errSim)
		t.FailNow()
	}
	t.Logf("got sim %v", raspiHw)
}*/

func TestPWMCalcStable(t *testing.T) {

	rf := govattu.RfSettings{Pwmc: 192, Pwmr: 2000, Pwm: 100}
	timing := rf.GetTiming()
	rf2, errSet := timing.GetSettings()
	if errSet != nil {
		t.Error(errSet)
	}

	fmt.Printf("Servo test timing= %#v\n", timing)
	fmt.Printf("Servo test rf2= pwmc=%v pwmr=%v pwm=%v\n", rf2.Pwmc, rf2.Pwmr, rf2.Pwm)
	fmt.Printf("Servo test final timing= %#v\n", rf2.GetTiming())

	rf.Pwmc = 3
	for pwmr := 2; pwmr < 1000; pwmr += 300 {
		for pwm := 1; pwm < pwmr-2; pwm++ {
			rf.Pwmr = uint32(pwmr)
			rf.Pwm = uint32(pwm)
			timing = rf.GetTiming()
			rf2, errSet = timing.GetSettings()
			if errSet != nil {
				t.Error(errSet)
			}

			hiErr, loErr := timing.DeviationPercent(rf2.GetTiming())
			fmt.Printf("hiErr=%v loErr=%v\n", hiErr, loErr)

			/*
				if (15 < hiErr) || (15 < loErr) {
					t.Error(fmt.Sprintf("Pulse timing not stable %#v -> %#v -> %#v\n", rf, timing, rf2))
					t.FailNow()
				}
			*/
			/*
				if !rf.EqualNear(rf2) {
					t.Error(fmt.Sprintf("Pulse timing not stable %#v -> %#v -> %#v\n", rf, timing, rf2))
					timing2 := rf2.GetTiming()
					fmt.Printf("Error on hi %v  and lo %v\n", timing.On-timing2.On, timing.Off-timing2.Off)
					t.FailNow()
				} else {
					fmt.Printf("\nrf=%#v  timing=%#v\n", rf, timing)
				}
			*/
		}
	}

}
