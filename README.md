# web-crawler
 Crawl a website and save marked URL path(s) to DB

### Summary:
Crawler will crawl the provided Base URL and fetch all the valid hrefs on the page.
Unseen hrefs will be added to a unique queue for fetching hrefs in them.
Crawler will save the paths which are to be monitored (from models) or marked (from cmd arg).

### Usage:

    webcrawler -baseurl <url> -db-dsn "<dsn>" [OPTIONS]

    -baseurl string
        Absolute base URL to crawl (required).
        E.g. <http/https>://<domain-name>
    -days int
        Days past which monitored URLs should be updated (default 1)
    -db-dsn string
        PostgreSQL DSN (required)
    -idle-time string
        Idle time after which crawler quits when queue is empty.
        Min: 1s (default "10s")
    -ignore string
        Comma ',' seperated string of url paths to ignore.
    -murls string
        Comma ',' seperated string of marked url paths to save/update.
        When empty, crawler will update monitored URLs from the model.
    -n int
        Number of crawlers to invoke (default 10)
    -req-delay string
        Delay between subsequent requests.
        Min: 1ms (default "50ms")
    -retry int
        Number of times to retry failed GET requests.
        With retry=2, crawlers will retry the failed GET urls
        twice after initial failure. (default 2)
    -v  Display app version

  Note: 
   - Crawler will ignore the hrefs that begins with "file:", "javascript:", "mailto:", "tel:", "#", "data:"
   - Marking URLs with -murls option will set is_monitored=true in models.