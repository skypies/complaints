// As close to a no-op service worker we can manage, that still calls fetch()

// https://developers.google.com/web/fundamentals/primers/service-workers/

self.addEventListener('install', function(event) {
  // Perform install steps
  var CACHE_NAME = 'jetnoise-cache-v1';
    var urlsToCache = [
        '/static/serfr.css',
    ];

    self.addEventListener('install', function(event) {
        // Perform install steps
        event.waitUntil(
            caches.open(CACHE_NAME)
                .then(function(cache) {
                    console.log('Opened cache');
                    return cache.addAll(urlsToCache);
                })
        );
    });
});


self.addEventListener('fetch', function(event) {
    event.respondWith(
        caches.match(event.request)
            .then(function(response) {
                // Cache hit - return response
                if (response) {
                    return response;
                }
                return fetch(event.request);
            })
    );
});
