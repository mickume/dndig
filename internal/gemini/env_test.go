package gemini

import "os"

func envHasGoogleKey() bool {
	for _, k := range []string{"GOOGLE_GENERATIVE_AI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY", "GOOGLE_APPLICATION_CREDENTIALS", "GOOGLE_CLOUD_PROJECT", "CLOUDSDK_CORE_PROJECT"} {
		if os.Getenv(k) != "" {
			return true
		}
	}
	return false
}
