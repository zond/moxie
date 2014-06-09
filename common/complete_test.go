package common

import "testing"

func TestComplete(t *testing.T) {
	var c *CompleteNode
	c = c.Insert([]byte("hejsan"))
	c = c.Insert([]byte("hepple"))
	c = c.Insert([]byte("hejkompis"))
	c = c.Insert([]byte("abab"))
	c = c.Insert([]byte("abrakadabra"))
	c = c.Insert([]byte("examine"))
	c = c.Insert([]byte("exa"))
	assertMatch(t, c, "abraka", "abrakadabra", true)
	assertMatch(t, c, "ab", "", false)
	assertMatch(t, c, "abab", "abab", true)
	assertMatch(t, c, "ex", "exa", true)
	assertMatch(t, c, "exa", "exa", true)
	assertMatch(t, c, "exam", "examine", true)
}

func assertMatch(t *testing.T, c *CompleteNode, toComplete, expected string, expectedFound bool) {
	completed, found := c.Complete([]byte(toComplete))
	if found != expectedFound || string(completed) != expected {
		t.Fatalf("Wanted %#v, %v, got %#v, %v", expected, expectedFound, string(completed), found)
	}
}
