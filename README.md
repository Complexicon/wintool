# wintool

This tool was primarily written to create a fully functioning Windows install from linux without downloading any isos. 
Through this project however, i created some utilities some may find interesting like mounting udf images as an go-fs
currently there are following submodules:

- httpreader -> use a single file from a http server (that supports 206 Partial Content) as an io.ReadAt interface
- msiso -> programmatically get iso links for windows directly from microsoft
- udf -> work with iso images as a go-fs
- wimfs work with wim-files/streams as a go-fs

## remarks
this project is not complete in any way shape or form and may grow beyond the initial intent, also this thing is mostly thrown together for fun and learning :)