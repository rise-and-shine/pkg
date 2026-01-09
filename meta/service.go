package meta

import "sync"

var (
	serviceName    string    //nolint:gochecknoglobals // for minimizing dependency injection across codebase
	serviceVersion string    //nolint:gochecknoglobals // for minimizing dependency injection across codebase
	once           sync.Once //nolint:gochecknoglobals // ensures SetServiceInfo is called once
)

// SetServiceInfo sets the global service name and version.
// This should be called once at application startup.
// Subsequent calls are ignored.
func SetServiceInfo(name, version string) {
	once.Do(func() {
		serviceName = name
		serviceVersion = version
	})
}

// ServiceName returns the global service name.
func ServiceName() string {
	return serviceName
}

// ServiceVersion returns the global service version.
func ServiceVersion() string {
	return serviceVersion
}
