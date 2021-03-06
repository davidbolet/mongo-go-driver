// Copyright (C) MongoDB, Inc. 2017-present.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at http://www.apache.org/licenses/LICENSE-2.0

package description

import (
	"fmt"

	"go.mongodb.org/mongo-driver/mongo/address"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// Topology represents a description of a mongodb topology
type Topology struct {
	Servers               []Server
	SetName               string
	Kind                  TopologyKind
	SessionTimeoutMinutes uint32
	CompatibilityErr      error
}

// Server returns the server for the given address. Returns false if the server
// could not be found.
func (t Topology) Server(addr address.Address) (Server, bool) {
	for _, server := range t.Servers {
		if server.Addr.String() == addr.String() {
			return server, true
		}
	}
	return Server{}, false
}

// TopologyDiff is the difference between two different topology descriptions.
type TopologyDiff struct {
	Added   []Server
	Removed []Server
}

// DiffTopology compares the two topology descriptions and returns the difference.
func DiffTopology(old, new Topology) TopologyDiff {
	var diff TopologyDiff

	oldServers := make(map[string]bool)
	for _, s := range old.Servers {
		oldServers[s.Addr.String()] = true
	}

	for _, s := range new.Servers {
		addr := s.Addr.String()
		if oldServers[addr] {
			delete(oldServers, addr)
		} else {
			diff.Added = append(diff.Added, s)
		}
	}

	for _, s := range old.Servers {
		addr := s.Addr.String()
		if oldServers[addr] {
			diff.Removed = append(diff.Removed, s)
		}
	}

	return diff
}

// HostlistDiff is the difference between a topology and a host list.
type HostlistDiff struct {
	Added   []string
	Removed []string
}

// DiffHostlist compares the topology description and host list and returns the difference.
func (t Topology) DiffHostlist(hostlist []string) HostlistDiff {
	var diff HostlistDiff

	oldServers := make(map[string]bool)
	for _, s := range t.Servers {
		oldServers[s.Addr.String()] = true
	}

	for _, addr := range hostlist {
		if oldServers[addr] {
			delete(oldServers, addr)
		} else {
			diff.Added = append(diff.Added, addr)
		}
	}

	for addr := range oldServers {
		diff.Removed = append(diff.Removed, addr)
	}

	return diff
}

// String implements the Stringer interface
func (t Topology) String() string {
	var serversStr string
	for _, s := range t.Servers {
		serversStr += "{ " + s.String() + " }, "
	}
	return fmt.Sprintf("Type: %s, Servers: [%s]", t.Kind, serversStr)
}

// Equal compares two topology descriptions and returns true if they are equal
func (t Topology) Equal(other Topology) bool {

	diff := DiffTopology(t, other)
	if len(diff.Added) != 0 || len(diff.Removed) != 0 {
		return false
	}

	if t.Kind != other.Kind {
		return false
	}

	topoServers := make(map[string]Server)
	for _, s := range t.Servers {
		topoServers[s.Addr.String()] = s
	}

	otherServers := make(map[string]Server)
	for _, s := range other.Servers {
		otherServers[s.Addr.String()] = s
	}

	if len(topoServers) != len(otherServers) {
		return false
	}

	for _, server := range topoServers {
		otherServer := otherServers[server.Addr.String()]

		if !server.Equal(otherServer) {
			return false
		}
	}

	return true
}

// HasReadableServer returns true if a topology has a server available for reading
// based on the specified read preference. Single and sharded topologies only require an
// available server, while replica sets require an available server that has a kind
// compatible with the given read preference mode.
func (t Topology) HasReadableServer(mode readpref.Mode) bool {
	switch t.Kind {
	case Single, Sharded:
		return hasAvailableServer(t.Servers, 0)
	case ReplicaSetWithPrimary:
		return hasAvailableServer(t.Servers, mode)
	case ReplicaSetNoPrimary, ReplicaSet:
		if mode == readpref.PrimaryMode {
			return false
		}
		// invalid read preference
		if !mode.IsValid() {
			return false
		}

		return hasAvailableServer(t.Servers, mode)
	}
	return false
}

// HasWritableServer returns true if a topology has a server available for writing
func (t Topology) HasWritableServer() bool {
	return t.HasReadableServer(readpref.PrimaryMode)
}

// hasAvailableServer returns true if any servers are available based on
// the read preference.
func hasAvailableServer(servers []Server, mode readpref.Mode) bool {
	switch mode {
	case readpref.PrimaryMode:
		for _, s := range servers {
			if s.Kind == RSPrimary {
				return true
			}
		}
		return false
	case readpref.PrimaryPreferredMode, readpref.SecondaryPreferredMode, readpref.NearestMode:
		for _, s := range servers {
			if s.Kind == RSPrimary || s.Kind == RSSecondary {
				return true
			}
		}
		return false
	case readpref.SecondaryMode:
		for _, s := range servers {
			if s.Kind == RSSecondary {
				return true
			}
		}
		return false
	}

	// read preference is not specified
	for _, s := range servers {
		switch s.Kind {
		case Standalone,
			RSMember,
			RSPrimary,
			RSSecondary,
			RSArbiter,
			RSGhost,
			Mongos:
			return true
		}
	}

	return false
}
