package main

import (
	"fmt"
	"io/ioutil"
	. "launchpad.net/gocheck"
	"launchpad.net/juju-core/environs/agent"
	"launchpad.net/juju-core/environs/dummy"
	"launchpad.net/juju-core/environs/tools"
	"launchpad.net/juju-core/state"
	"launchpad.net/juju-core/version"
	"net/http"
	"path/filepath"
	"time"
)

var _ = Suite(&UpgraderSuite{})

type UpgraderSuite struct {
	oldVersion version.Binary
	agentSuite
}

func (s *UpgraderSuite) SetUpTest(c *C) {
	s.JujuConnSuite.SetUpTest(c)
	s.oldVersion = version.Current
}

func (s *UpgraderSuite) TearDownTest(c *C) {
	version.Current = s.oldVersion
	s.JujuConnSuite.TearDownTest(c)
}

func (s *UpgraderSuite) TestUpgraderStop(c *C) {
	u := s.startUpgrader(c, &state.Tools{Binary: version.Current})
	err := u.Stop()
	c.Assert(err, IsNil)
}

type proposal struct {
	version    string
	devVersion bool
}

// TODO(fwereade): Here be dragons. All sorts of state is smeared across
var upgraderTests = []struct {
	about      string
	current    string   // current version.
	upload     []string // Upload these tools versions.
	propose    string   // Propose this version...
	devVersion bool     // ... with devVersion set to this.

	// upgradeTo is blank if nothing should happen.
	upgradeTo string
}{{
	about:   "propose with no possible candidates",
	current: "2.0.0",
	propose: "2.2.0",
}, {
	about:   "propose with same candidate as current",
	current: "2.0.0",
	upload:  []string{"2.0.0"},
	propose: "2.4.0",
}, {
	about:   "propose development version when !devVersion",
	current: "2.0.0",
	upload:  []string{"2.1.0"},
	propose: "2.4.0",
}, {
	about:      "propose development version when devVersion",
	current:    "2.0.0",
	upload:     []string{"2.1.0"},
	propose:    "2.4.0",
	devVersion: true,
	upgradeTo:  "2.1.0",
}, {
	about:     "propose release version when !devVersion",
	current:   "2.1.0",
	upload:    []string{"2.0.0"},
	propose:   "2.4.0",
	upgradeTo: "2.0.0",
}, {
	about:     "propose with higher available candidates",
	current:   "2.0.0",
	upload:    []string{"2.4.0", "2.5.0", "2.6.0"},
	propose:   "2.4.0",
	upgradeTo: "2.4.0",
}, {
	about:     "propose exact available version",
	current:   "2.4.0",
	upload:    []string{"2.6.0"},
	propose:   "2.6.0",
	upgradeTo: "2.6.0",
}, {
	about:     "propose downgrade",
	current:   "2.6.0",
	upload:    []string{"2.5.0"},
	propose:   "2.5.0",
	upgradeTo: "2.5.0",
}, {
	about:     "upgrade with no proposal",
	current:   "2.6.0",
	upload:    []string{"2.5.0"},
	upgradeTo: "2.5.0",
},
}

func (s *UpgraderSuite) TestUpgrader(c *C) {
	currentTools := s.primeTools(c, version.MustParseBinary("2.0.0-foo-bar"))
	// Remove the tools from the storage so that we're sure that the
	// uploader isn't trying to fetch them.
	resp, err := http.Get(currentTools.URL)
	c.Assert(err, IsNil)
	err = agent.UnpackTools(s.DataDir(), currentTools, resp.Body)
	c.Assert(err, IsNil)
	s.removeVersion(c, currentTools.Binary)

	var (
		u            *Upgrader
		upgraderDone <-chan error
	)

	defer func() {
		if u != nil {
			c.Assert(u.Stop(), IsNil)
		}
	}()

	for i, test := range upgraderTests {
		c.Logf("\ntest %d: %s", i, test.about)
		s.removeTools(c)
		uploaded := make(map[version.Number]*state.Tools)
		for _, v := range test.upload {
			vers := version.Current
			vers.Number = version.MustParse(v)
			tools := s.uploadTools(c, vers)
			uploaded[vers.Number] = tools
		}
		if test.current == "" {
			panic("incomplete test setup: current is mandatory")
		}
		version.Current.Number = version.MustParse(test.current)
		currentTools, err = agent.ReadTools(s.DataDir(), version.Current)
		c.Assert(err, IsNil)
		if u == nil {
			u = s.startUpgrader(c, currentTools)
		}
		if test.propose != "" {
			s.proposeVersion(c, version.MustParse(test.propose), test.devVersion)
			s.State.StartSync()
		}
		if test.upgradeTo == "" {
			s.State.StartSync()
			assertNothingHappens(c, upgraderDone)
		} else {
			ug := waitDeath(c, u)
			tools := uploaded[version.MustParse(test.upgradeTo)]
			c.Check(ug.NewTools, DeepEquals, tools)
			c.Check(ug.OldTools.Binary, Equals, version.Current)
			c.Check(ug.DataDir, Equals, s.DataDir())
			c.Check(ug.AgentName, Equals, "testagent")

			// Check that the upgraded version was really downloaded.
			path := agent.SharedToolsDir(s.DataDir(), tools.Binary)
			data, err := ioutil.ReadFile(filepath.Join(path, "jujud"))
			c.Assert(err, IsNil)
			c.Assert(string(data), Equals, "jujud contents "+tools.Binary.String())

			u, upgraderDone = nil, nil
		}
	}
}

var delayedStopTests = []struct {
	about             string
	upgraderKillDelay time.Duration
	storageDelay      time.Duration
	propose           string
	err               string
}{{
	about:             "same version",
	upgraderKillDelay: time.Second,
	propose:           "2.0.3",
}, {
	about:             "same version found for higher proposed version",
	upgraderKillDelay: time.Second,
	propose:           "2.0.4",
}, {
	about:             "no appropriate version found",
	upgraderKillDelay: time.Second,
	propose:           "2.0.3",
}, {
	about:             "time out",
	propose:           "2.0.6",
	storageDelay:      time.Second,
	upgraderKillDelay: 10 * time.Millisecond,
	err:               "upgrader aborted download.*",
}, {
	about:             "successful upgrade",
	upgraderKillDelay: time.Second,
	propose:           "2.0.6",
	// enough delay that the stop will probably arrive before the
	// tools have downloaded, thus checking that the
	// upgrader really did wait for the download.
	storageDelay: 5 * time.Millisecond,
	err:          `must restart: an agent upgrade is available`,
}, {
	about:             "fetch error",
	upgraderKillDelay: time.Second,
	propose:           "2.0.7",
},
}

func (s *UpgraderSuite) TestDelayedStop(c *C) {
	defer dummy.SetStorageDelay(0)
	tools := s.primeTools(c, version.MustParseBinary("2.0.3-foo-bar"))
	s.uploadTools(c, version.MustParseBinary("2.0.5-foo-bar"))
	s.uploadTools(c, version.MustParseBinary("2.0.6-foo-bar"))
	s.uploadTools(c, version.MustParseBinary("2.0.6-foo-bar"))
	s.uploadTools(c, version.MustParseBinary("2.0.7-foo-bar"))
	s.poisonVersion(version.MustParseBinary("2.0.7-foo-bar"))

	for i, test := range delayedStopTests {
		c.Logf("%d. %v", i, test.about)
		upgraderKillDelay = test.upgraderKillDelay
		dummy.SetStorageDelay(test.storageDelay)
		proposed := version.MustParse(test.propose)
		s.proposeVersion(c, proposed, true)
		u := s.startUpgrader(c, tools)
		t0 := time.Now()
		err := u.Stop()
		d := time.Now().Sub(t0)
		if d > 100*time.Millisecond {
			c.Errorf("upgrader took took too long: %v", d)
		}
		if test.err == "" {
			c.Check(err, IsNil)
		} else {
			c.Check(err, ErrorMatches, test.err)
		}
	}
}

func (s *UpgraderSuite) poisonVersion(vers version.Binary) {
	name := tools.StorageName(vers)
	dummy.Poison(s.Conn.Environ.Storage(), name, fmt.Errorf("poisoned file"))
}

func (s *UpgraderSuite) removeVersion(c *C, vers version.Binary) {
	name := tools.StorageName(vers)
	err := s.Conn.Environ.Storage().Remove(name)
	c.Assert(err, IsNil)
}

func (s *UpgraderSuite) TestUpgraderReadyErrorUpgrade(c *C) {
	currentTools := s.primeTools(c, version.MustParseBinary("2.0.2-foo-bar"))
	ug := &UpgradeReadyError{
		AgentName: "foo",
		OldTools:  &state.Tools{Binary: version.MustParseBinary("2.0.0-foo-bar")},
		NewTools:  currentTools,
		DataDir:   s.DataDir(),
	}
	err := ug.ChangeAgentTools()
	c.Assert(err, IsNil)
	d := agent.ToolsDir(s.DataDir(), "foo")
	data, err := ioutil.ReadFile(filepath.Join(d, "jujud"))
	c.Assert(err, IsNil)
	c.Assert(string(data), Equals, "jujud contents 2.0.2-foo-bar")
}

func assertNothingHappens(c *C, upgraderDone <-chan error) {
	select {
	case got := <-upgraderDone:
		c.Fatalf("expected nothing to happen, got %v", got)
	case <-time.After(100 * time.Millisecond):
	}
}

func assertEvent(c *C, event <-chan string, want string) {
	select {
	case got := <-event:
		c.Assert(got, Equals, want)
	case <-time.After(500 * time.Millisecond):
		c.Fatalf("no event received; expected %q", want)
	}
}

// startUpgrader starts the upgrader using the given machine,
// expecting to see it set the given agent tools.
func (s *UpgraderSuite) startUpgrader(c *C, expectTools *state.Tools) *Upgrader {
	as := testAgentState(make(chan *state.Tools))
	u := NewUpgrader(s.State, as, s.DataDir())
	select {
	case tools := <-as:
		c.Assert(tools, DeepEquals, expectTools)
	case <-time.After(500 * time.Millisecond):
		c.Fatalf("upgrader did not set agent tools")
	}
	return u
}

func waitDeath(c *C, u *Upgrader) *UpgradeReadyError {
	done := make(chan error, 1)
	go func() {
		done <- u.Wait()
	}()
	select {
	case err := <-done:
		c.Assert(err, FitsTypeOf, &UpgradeReadyError{})
		return err.(*UpgradeReadyError)
	case <-time.After(500 * time.Millisecond):
		c.Fatalf("upgrader did not die as expected")
	}
	panic("unreachable")
}

type testAgentState chan *state.Tools

func (as testAgentState) SetAgentTools(tools *state.Tools) error {
	t := *tools
	as <- &t
	return nil
}

func (as testAgentState) Tag() string {
	return "testagent"
}

func (as testAgentState) Life() state.Life {
	panic("unimplemented")
}

func (as testAgentState) SetMongoPassword(string) error {
	panic("unimplemented")
}
