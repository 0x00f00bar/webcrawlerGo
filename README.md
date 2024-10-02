# web-crawler
 Crawl a website and save marked URL path(s) to DB

### Summary:
 Crawler will crawl the provided Base URL and fetch all the valid hrefs on the page.
 Unseen hrefs will be added to a unique queue for fetching hrefs in them.
 Crawler will save the paths which are to be monitored (from models) or marked (from cmd args).
