Little program that follows my location in Home Assistant and searches Sondehub for nearby sondes to tell me about them.

Also my first experiment in coding in Go. 🤔

It'll probably work for you if you're running Home Assistant and your mobile device is pushing it's location.  It's read from the Person entity, so your mobile device has to be associated with the person entity (which caught me out).

When it finds a matching sonde, it'll notify the Person entity back in Home Assistant.  The notification has a URL which links to Sondehub.

May you find many more lucky sondes!
