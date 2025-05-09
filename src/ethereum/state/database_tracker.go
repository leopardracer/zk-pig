package state

import (
	gethcommon "github.com/ethereum/go-ethereum/common"
	gethstate "github.com/ethereum/go-ethereum/core/state"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
)

// StateAccessTracker is a state database that tracks the state access (account, storage, and bytecode) during block execution.
type AccessTrackerDatabase struct {
	gethstate.Database

	trackers *AccessTrackerManager

	// TODO: remove the current tracker that should be useless
	// as we can use native go-ethereum witness
	currentTracker *AccessTracker
}

// NewAccessTrackerDatabase creates a new state database that tracks the state access during block execution.
func NewAccessTrackerDatabase(db gethstate.Database, trackers *AccessTrackerManager) *AccessTrackerDatabase {
	return &AccessTrackerDatabase{
		Database: db,
		trackers: trackers,
	}
}

// Reader implements the gethstate.Database interface.
func (db *AccessTrackerDatabase) Reader(stateRoot gethcommon.Hash) (gethstate.Reader, error) {
	reader, err := db.Database.Reader(stateRoot)
	if err != nil {
		return nil, err
	}

	tracker := newStateAccessTracker()
	db.trackers.SetTracker(stateRoot, tracker)
	db.currentTracker = tracker

	return newStateAccessTrackerReader(reader, tracker), nil
}

type AccountAccessTracker struct {
	Account *gethtypes.StateAccount
	Storage map[gethcommon.Hash]gethcommon.Hash
}
type AccessTracker struct {
	Accounts map[gethcommon.Address]*AccountAccessTracker
}

type AccessTrackerManager struct {
	trackers map[gethcommon.Hash]*AccessTracker
}

func NewAccessTrackerManager() *AccessTrackerManager {
	return &AccessTrackerManager{
		trackers: make(map[gethcommon.Hash]*AccessTracker),
	}
}

func (m *AccessTrackerManager) GetAccessTracker(stateRoot gethcommon.Hash) *AccessTracker {
	if tracker, ok := m.trackers[stateRoot]; ok {
		return tracker
	}
	return nil
}

func (m *AccessTrackerManager) SetTracker(stateRoot gethcommon.Hash, tracker *AccessTracker) *AccessTracker {
	m.trackers[stateRoot] = tracker
	return tracker
}

func (m *AccessTrackerManager) DeleteTracker(stateRoot gethcommon.Hash) {
	delete(m.trackers, stateRoot)
}

func (m *AccessTrackerManager) Clear() {
	m.trackers = make(map[gethcommon.Hash]*AccessTracker)
}

func newStateAccessTracker() *AccessTracker {
	return &AccessTracker{
		Accounts: make(map[gethcommon.Address]*AccountAccessTracker),
	}
}

// stateAccessTrackerReader is a state reader that tracks the state access (account and storage) during the read operation.
type stateAccessTrackerReader struct {
	reader gethstate.Reader

	tracker *AccessTracker
}

func newStateAccessTrackerReader(reader gethstate.Reader, tracker *AccessTracker) *stateAccessTrackerReader {
	return &stateAccessTrackerReader{
		reader:  reader,
		tracker: tracker,
	}
}

// Account implementing Reader interface, retrieving the account associated with
// a particular address.
func (r *stateAccessTrackerReader) Account(addr gethcommon.Address) (*gethtypes.StateAccount, error) {
	account, err := r.reader.Account(addr)
	if err != nil {
		return nil, err
	}

	_, ok := r.tracker.Accounts[addr]
	if !ok {
		r.tracker.Accounts[addr] = &AccountAccessTracker{
			Storage: make(map[gethcommon.Hash]gethcommon.Hash),
		}

		if account != nil {
			r.tracker.Accounts[addr].Account = account.Copy()
		}
	}

	return account, nil
}

// Storage implementing Reader interface, retrieving the storage slot associated
// with a particular account address and slot key.
func (r *stateAccessTrackerReader) Storage(addr gethcommon.Address, slot gethcommon.Hash) (gethcommon.Hash, error) {
	value, err := r.reader.Storage(addr, slot)
	if err != nil {
		return gethcommon.Hash{}, err
	}

	if _, ok := r.tracker.Accounts[addr]; !ok {
		r.tracker.Accounts[addr] = &AccountAccessTracker{
			Storage: make(map[gethcommon.Hash]gethcommon.Hash),
		}
	}

	if _, ok := r.tracker.Accounts[addr].Storage[slot]; !ok {
		r.tracker.Accounts[addr].Storage[slot] = value
	}

	return value, nil
}

// Code implementing Reader interface, retrieving the code associated with
// a particular account address.
func (r *stateAccessTrackerReader) Code(addr gethcommon.Address, codeHash gethcommon.Hash) ([]byte, error) {
	code, err := r.reader.Code(addr, codeHash)
	return code, err
}

// CodeSize implementing Reader interface, retrieving the size of the code associated with
// a particular account address.
func (r *stateAccessTrackerReader) CodeSize(addr gethcommon.Address, codeHash gethcommon.Hash) (int, error) {
	size, err := r.reader.CodeSize(addr, codeHash)
	return size, err
}
