{{define "js-map-trail"}} // Depends on: .Center (geo.Latlong), and .Zoom (int)

// OK outcomes
//	ClassB_NotInRange = 0
//	ClassB_Above = 1
//	ClassB_Within = 2
//	ClassB_Below = 3
// Violations
//	ClassB_Within_TooFast = 10
//	ClassB_Below_TooFast = 11

function localOverlay() {
    var legend = document.getElementById('legend');
    var div = document.createElement('div');
    div.innerHTML = {{.Legend}};
    legend.appendChild(div);
    
    mapstrack = {{.MapsTrack}}
    
    var infowindow = new google.maps.InfoWindow({ content: "holding..." });

    for (var i in mapstrack) {
        var imgurl = "/static/dot-green.png";
        //if (mapstrack[i].violation > 0) { imgurl = "/static/dot-red.png"; }
        var infostring = '<div><pre>' + mapstrack[i].debug + '</pre></div>';
        var marker = new google.maps.Marker({
            position: mapstrack[i].pos,
            map: map,
            title: mapstrack[i].id,
            icon: imgurl,
            html: infostring,
        });
        marker.addListener('click', function(){
            infowindow.setContent(this.html),
            infowindow.open(map, this);
        });
    }

    fatrack = {{.FlightawareTrack}}

    for (var i in fatrack) {
        var imgurl = "/static/dot-blue.png";
        if (fatrack[i].violation > 0) { imgurl = "/static/dot-red-large.gif"; }
        var infostring = '<div><pre>' + fatrack[i].debug + '</pre></div>';
        var marker = new google.maps.Marker({
            position: fatrack[i].pos,
            map: map,
            title: fatrack[i].id,
            icon: imgurl,
            html: infostring,
        });
        marker.addListener('click', function(){
            infowindow.setContent(this.html),
            infowindow.open(map, this);
        });
    }

    adsbtrack = {{.ADSBTrack}}

    for (var i in adsbtrack) {
        var imgurl = "/static/dot-yellow.png";
        if (adsbtrack[i].violation > 0) { imgurl = "/static/dot-red-large.gif"; }
        var infostring = '<div><pre>' + adsbtrack[i].debug + '</pre></div>';
        var marker = new google.maps.Marker({
            position: adsbtrack[i].pos,
            map: map,
            title: adsbtrack[i].id,
            icon: imgurl,
            html: infostring,
        });
        marker.addListener('click', function(){
            infowindow.setContent(this.html),
            infowindow.open(map, this);
        });
    }

    skimtrack = {{.SkimTrack}}

    for (var i in skimtrack) {
        var imgurl = "/static/dot-blue.png";
        if (skimtrack[i].violation > 0) { imgurl = "/static/dot-red-large.gif"; }
        var infostring = '<div><pre>' + skimtrack[i].debug + '</pre></div>';
        var marker = new google.maps.Marker({
            position: skimtrack[i].pos,
            map: map,
            title: skimtrack[i].id,
            icon: imgurl,
            html: infostring,
        });
        marker.addListener('click', function(){
            infowindow.setContent(this.html),
            infowindow.open(map, this);
        });
    }

}
{{end}}
