package fetcher

// UserAgent is the user agent we send with requests. We mimic a mobile browser, because the
// Lodestone detects that and sends a mobile page, which is much smaller than the desktop one.
const UserAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 9_1 like Mac OS X) AppleWebKit/601.1.46 (KHTML, like Gecko) Version/9.0 Mobile/13B143 Safari/601.1"

// LodestoneBaseURL is the base URL for requests to the Lodestone.
var LodestoneBaseURL = "https://na.finalfantasyxiv.com/lodestone/"
