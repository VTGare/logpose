package cards

import (
	"errors"
	"time"
)

// Loaded is the card database with when its caches were last brought up to date.
type Loaded struct {
	DB *DB
	// Fetched is when the catalog was refreshed, and Checked when the library was.
	Fetched, Checked time.Time
}

// Load layers the card data. pack, OPBounty's catalog from LoadPack, goes first, then the cached
// tcgcsv catalog, which has newer sets, then the Limitless library, which has everything. Names
// from the logs go over all of it with DB.Name. pack can be nil and a path empty. A file that
// doesn't load is left out, and its error returned with what the others give.
func Load(pack *DB, catalogPath, libraryPath string) (Loaded, error) {
	l := Loaded{DB: NewDB()}
	if pack != nil {
		l.DB = pack.Clone()
	}
	var errs []error
	if catalogPath != "" {
		cat, err := LoadCatalog(catalogPath)
		if err == nil {
			cat.Apply(l.DB)
			l.Fetched = cat.Fetched
		}
		errs = append(errs, err)
	}
	if libraryPath != "" {
		lib, err := LoadLibrary(libraryPath)
		if err == nil {
			lib.Apply(l.DB)
			l.Checked = lib.Checked
		}
		errs = append(errs, err)
	}
	return l, errors.Join(errs...)
}

// Refreshed is what a cache refresh did. Saved is false when the cache was fresh, or when the
// refresh failed before fetching anything.
type Refreshed struct {
	Saved bool
	// Sets is how many set pages were fetched, and Cards how many cards the cache holds.
	Sets, Cards int
}

func saveRefreshed(c interface{ Save(path string) error }, path string, sets, cards int, err error) (Refreshed, error) {
	if sets == 0 && err != nil {
		return Refreshed{}, err
	}
	if serr := c.Save(path); serr != nil {
		return Refreshed{Sets: sets, Cards: cards}, errors.Join(err, serr)
	}
	return Refreshed{Saved: true, Sets: sets, Cards: cards}, err
}
