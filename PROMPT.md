I want to create a new application for my twitch stream.  

The applications purpose is to:
- sync my album list from my Discogs collection
- ensure that you follow Discogs API throttles to not go over limits
- display the name of the album, artist, song, and duration
- display how high the song topped in the charts and include the date it hit its peak
- sync the display with the duration of the length of the song


Things you need to consider:
- I will be playing the vinyl on my turntable and you won't know which actual song is playing
- Give me a simple interface to manage what is being displayed so I can skip the timer forward, backwards, or pause it
- Keep the display and managment interface on different dashboards

I'd like this application to use Go and Gin.
I'd like the storage to be done in mariadb, so you will have to help me with the commands to setup the database and tables.