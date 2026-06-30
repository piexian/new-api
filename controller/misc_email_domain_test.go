package controller

import "testing"

func TestIsEmailDomainAllowedExact(t *testing.T) {
	// No wildcards: strict equality only.
	wl := []string{"edu.cn", "gmail.com"}
	check := func(domain string, want bool) {
		t.Run("exact/"+domain, func(t *testing.T) {
			if got := isEmailDomainAllowed(domain, wl); got != want {
				t.Errorf("isEmailDomainAllowed(%q) = %v, want %v", domain, got, want)
			}
		})
	}
	check("edu.cn", true)
	check("gmail.com", true)
	check("buaa.edu.cn", false) // subdomain not covered without wildcard
	check("mail.gmail.com", false)
	check("EDU.CN", true) // case-insensitive
}

func TestIsEmailDomainAllowedOneLevelWildcard(t *testing.T) {
	wl := []string{"*.edu.cn"}
	check := func(domain string, want bool) {
		t.Run("1level/"+domain, func(t *testing.T) {
			if got := isEmailDomainAllowed(domain, wl); got != want {
				t.Errorf("isEmailDomainAllowed(%q) = %v, want %v", domain, got, want)
			}
		})
	}
	check("buaa.edu.cn", true) // 1 level matches
	check("tsinghua.edu.cn", true)
	check("edu.cn", false)         // bare domain, label count differs
	check("stu.hit.edu.cn", false) // 2 levels, only 1 wildcard configured
	check("fakedu.cn", false)      // suffix trick blocked (no leading dot)
}

func TestIsEmailDomainAllowedTwoLevelWildcard(t *testing.T) {
	wl := []string{"*.*.edu.cn"}
	check := func(domain string, want bool) {
		t.Run("2level/"+domain, func(t *testing.T) {
			if got := isEmailDomainAllowed(domain, wl); got != want {
				t.Errorf("isEmailDomainAllowed(%q) = %v, want %v", domain, got, want)
			}
		})
	}
	check("stu.hit.edu.cn", true)
	check("mail.sysu.edu.cn", true)
	check("buaa.edu.cn", false)  // 1 level, needs 2
	check("a.b.c.edu.cn", false) // 3 levels, needs 2
}

func TestIsEmailDomainAllowedComWildcards(t *testing.T) {
	wl := []string{"*.com", "*.*.com"}
	check := func(domain string, want bool) {
		t.Run("com/"+domain, func(t *testing.T) {
			if got := isEmailDomainAllowed(domain, wl); got != want {
				t.Errorf("isEmailDomainAllowed(%q) = %v, want %v", domain, got, want)
			}
		})
	}
	check("gmail.com", true)      // via *.com
	check("mail.gmail.com", true) // via *.*.com
	check("a.b.gmail.com", false) // 3 levels before com, no rule covers it
	check("example.org", false)   // suffix differs
	check("fakecom", false)       // suffix trick blocked
	check("GMAIL.COM", true)      // case-insensitive
	check("Mail.GMAIL.COM", true) // case-insensitive, 2 levels
}

func TestIsEmailDomainAllowedMixed(t *testing.T) {
	// Combined exact + 1-level + 2-level wildcards.
	wl := []string{"gmail.com", "edu.cn", "*.edu.cn", "*.*.edu.cn"}
	check := func(domain string, want bool) {
		t.Run("mixed/"+domain, func(t *testing.T) {
			if got := isEmailDomainAllowed(domain, wl); got != want {
				t.Errorf("isEmailDomainAllowed(%q) = %v, want %v", domain, got, want)
			}
		})
	}
	check("gmail.com", true)
	check("edu.cn", true)
	check("buaa.edu.cn", true)    // via *.edu.cn
	check("stu.hit.edu.cn", true) // via *.*.edu.cn
	check("a.b.c.edu.cn", false)  // 3 levels, no rule covers it
	check("163.com", false)
	check("fakedu.cn", false)
	check("GMAIL.COM", true) // case-insensitive
	check("Tsinghua.EDU.CN", true)
}
