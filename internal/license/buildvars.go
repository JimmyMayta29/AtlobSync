package license

// Variables inyectables en tiempo de compilación mediante -ldflags "-X ...".
// Nunca almacenar aquí la llave service_role; únicamente URL pública y anon key.
var (
	BuildSupabaseURL     = ""
	BuildSupabaseAnonKey = ""
	BuildRequireLicense  = ""
	BuildSupportPhone    = "940 781 298"
	BuildSupportWhatsApp = "51940781298"
)
