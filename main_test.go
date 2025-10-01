package main

import "testing"

func TestCalc(t *testing.T) {
    // Common setup and teardown

	t.Run("addition", func(t *testing.T) {
		t.Parallel()
		addgroup(t)
	})

	t.Run("multiplication", func(t *testing.T) {
		t.Parallel()
		multgroup(t)
	})
}

func addgroup(t *testing.T) {
    // addition-specific setup
    defer func() {
        // addition-specific teardown
    }()

    t.Run("1+1=2", func(t *testing.T) {
        if 1+1 != 2 {
            t.Fatal("want 2")
        }
    })

    t.Run("2+3=5", func(t *testing.T) {
        if 2+3 != 5 {
            t.Fatal("want 5")
        }
    })
}

func multgroup(t *testing.T) {
    // multiplication-specific setup
    defer func() {
        // multiplication-specific teardown
    }()

    t.Run("2*2=4", func(t *testing.T) {
        if 2*2 != 4 {
            t.Fatal("want 4")
        }
    })

    t.Run("3*3=9", func(t *testing.T) {
        if 3*3 != 9 {
            t.Fatal("want 9")
        }
    })
}
