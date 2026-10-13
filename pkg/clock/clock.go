package clock

import "time"

// The application's business date is independent of the server OS timezone.
var Zone = time.FixedZone("Asia/Shanghai", 8*60*60)

func Now() time.Time { return time.Now().In(Zone) }
