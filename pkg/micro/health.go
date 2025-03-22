package micro

import (
	"context"
	"net/http"
	"sync"
	"time"
)

// Health check management
func (a *App) AddHealthCheck(name string, check HealthCheck) {
	a.healthChecks[name] = check
}

func (a *App) healthHandler(w http.ResponseWriter, r *http.Request) {
	if len(a.healthChecks) == 0 {
		a.JSON(w, http.StatusOK, map[string]string{"status": "OK"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	results := make(map[string]interface{})
	var mu sync.Mutex
	var wg sync.WaitGroup

	for name, hc := range a.healthChecks {
		wg.Add(1)
		go func(name string, check HealthCheck) {
			defer wg.Done()

			err := check.Check(ctx)
			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				results[name] = map[string]interface{}{
					"status":    "unhealthy",
					"error":     err.Error(),
					"timestamp": time.Now().UTC(),
				}
			} else {
				results[name] = map[string]interface{}{
					"status":    "healthy",
					"timestamp": time.Now().UTC(),
				}
			}
		}(name, hc)
	}

	wg.Wait()

	if len(results) == 0 {
		a.JSON(w, http.StatusOK, map[string]string{"status": "OK"})
		return
	}

	status := http.StatusOK
	for _, result := range results {
		if result.(map[string]interface{})["status"] != "healthy" {
			status = http.StatusServiceUnavailable
			break
		}
	}

	response := map[string]interface{}{
		"status":   http.StatusText(status),
		"checks":   results,
		"duration": time.Since(ctx.Value("start_time").(time.Time)).String(),
	}

	a.JSON(w, status, response)
}
