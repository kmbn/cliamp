package playlist

import "testing"

func TestEmptyPlaylistCurrent(t *testing.T) {
	p := New()
	_, idx := p.Current()
	if idx != -1 {
		t.Fatalf("Current() on empty: idx = %d, want -1", idx)
	}
}

func TestEmptyPlaylistIndex(t *testing.T) {
	p := New()
	if got := p.Index(); got != -1 {
		t.Fatalf("Index() on empty = %d, want -1", got)
	}
}

func TestAddStation(t *testing.T) {
	p := New()
	p.Add(Track{Title: "Radio Jazz", Path: "http://jazz.example.com"})
	if p.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", p.Len())
	}
	tr, idx := p.Current()
	if tr.Title != "Radio Jazz" || idx != 0 {
		t.Fatalf("Current() = (%q, %d), want (Radio Jazz, 0)", tr.Title, idx)
	}
}

func TestReplaceLoadsStation(t *testing.T) {
	p := New()
	p.Replace([]Track{{Title: "Station A", Path: "http://a.example.com"}})
	tr, idx := p.Current()
	if tr.Title != "Station A" || idx != 0 {
		t.Fatalf("Current() = (%q, %d), want (Station A, 0)", tr.Title, idx)
	}
}

func TestReplaceResetsToStart(t *testing.T) {
	p := New()
	p.Add(Track{Title: "Old Station"})
	p.Replace([]Track{{Title: "New Station"}})
	tr, idx := p.Current()
	if tr.Title != "New Station" || idx != 0 {
		t.Fatalf("after Replace: got (%q, %d), want (New Station, 0)", tr.Title, idx)
	}
}

func TestReplaceWithEmpty(t *testing.T) {
	p := New()
	p.Add(Track{Title: "Station"})
	p.Replace(nil)
	if p.Len() != 0 {
		t.Fatalf("Len() = %d after Replace(nil), want 0", p.Len())
	}
	_, idx := p.Current()
	if idx != -1 {
		t.Fatalf("Current() after Replace(nil): idx = %d, want -1", idx)
	}
}

func TestSetTrackUpdatesInPlace(t *testing.T) {
	p := New()
	p.Add(Track{Title: "Station", Path: "http://old.example.com"})
	p.SetTrack(0, Track{Title: "Station", Path: "http://new.example.com"})
	tr, _ := p.Current()
	if tr.Path != "http://new.example.com" {
		t.Fatalf("SetTrack: path = %q, want http://new.example.com", tr.Path)
	}
}

func TestNextAndPrevReturnFalseForSingleStation(t *testing.T) {
	p := New()
	p.Add(Track{Title: "Radio"})
	if _, ok := p.Next(); ok {
		t.Fatal("Next() past single station = true, want false")
	}
	if _, ok := p.Prev(); ok {
		t.Fatal("Prev() before single station = true, want false")
	}
}
