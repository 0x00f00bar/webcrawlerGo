# webcrawlerGo v0.7.0
 Crawl a website and save marked URL's contents to DB

### Summary:
Crawler will crawl the provided Base URL and fetch all the valid hrefs on the page.
Unseen hrefs will be added to a unique queue for fetching hrefs in them.
Crawler will save the paths which are to be monitored (from models) or marked (from cmd arg).
Crawler respects the robots.txt of the website being parsed.

Can use PostgreSQL when provided else will open a local sqlite3 database.

### Usage:

    webcrawler -baseurl <url> -db-dsn "<dsn>" [OPTIONS]

    -baseurl string
        Absolute base URL to crawl (required).
        E.g. <http/https>://<domain-name>
    -date string
        Cut-off date upto which the latest crawled pages will be saved to disk.
        Format: YYYY-MM-DD. Applicable only with 'save' flag.
        (default "2024-10-17")
    -days int
        Days past which monitored URLs should be updated (default 1)
    -db-dsn string
        DSN string to database.
        Supported DSN: PostgreSQL DSN (optional).
        When empty crawler will use sqlite3 driver.
    -db2disk
        Use this flag to write the latest crawled content to disk.
        Customise using arguments 'path' and 'date'.
        Crawler will exit after saving to disk.
    -idle-time string
        Idle time after which crawler quits when queue is empty.
        Min: 1s (default "10s")
    -ignore string
        Comma ',' seperated string of url patterns to ignore.
    -murls string
        Comma ',' seperated string of marked url paths to save/update.
        If the marked path is unmonitored in the database, the crawler
        will mark the URL as monitored.
        When empty, crawler will update monitored URLs from the model.
    -n int
        Number of crawlers to invoke (default 10)
    -path string
        Output path to save the content of crawled web pages.
        Applicable only with 'save' flag. (default "./OUT/2024-10-17_15-03-37")
    -req-delay string
        Delay between subsequent requests.
        Min: 1ms (default "50ms")
    -retry int
        Number of times to retry failed GET requests.
        With retry=2, crawlers will retry the failed GET urls
        twice after initial failure. (default 2)
    -ua string
        User-Agent string to use while crawling
        (default "webcrawlerGo/v0.7.0 - Web crawler in Go")
    -v   Display app version

  Note: 
   - Crawler will ignore the hrefs that begins with "file:", "javascript:", "mailto:", "tel:", "#", "data:"
   - Marking URLs with -murls option will set is_monitored=true in models.
   - Use -ignore option to ignore any pattern in url path, for e.g. to ignore paths with pdf files add '.pdf' to ignore
   list

