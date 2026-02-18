package arr

import "testing"

func TestBuildQueueDeleteURL(t *testing.T) {
	got := BuildQueueDeleteURL("http://localhost:7878/", "123", QueueDeleteOptions{
		RemoveFromClient: true,
		Blocklist:        false,
		SkipRedownload:   true,
		ChangeCategory:   true,
	})

	want := "http://localhost:7878/api/v3/queue/123?removeFromClient=true&blocklist=false&skipRedownload=true&changeCategory=true"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
