setColdStoragePurgeDate usage and info.

The purpose of this program is to set the purge date of archives in cold storage to a new date based on
two parameters: a baseline date and the number of days, N, after the baseline date to set the purge date to.

COMMAND LINE PARAMETERS
1. [-b date ] Baseline date . Date format: either TODAY or MM-DD-YYYY
2. [-d N ] Number of days in future after baseline date for new purge date.
3. [-t ] test : whether or not to really change the purge dates, or just output the number of archives
	that would be changed. Default is 'false'.
4. [-a ] all : sets date for all archives in cold storage to the date, not just those that have a purge date
	later than N days later than baseline). Default is 'false'
5. [-s] Skip destinations that report having zero cold storage bytes. Default is 'false'.
6. [-help] Show help.

Example command:
> ./setColdStoragePurgeDate -b 05-12-2016 -d 30 -t -s
 The example command above would use May 5, 2016 as the baseline date. The new archive expiration date would be 30 days after May 5, 2016.
 The program would run in test mode only (and not actually change the archive expiration dates). Finally, the program would
 skips any destination in the environment that reports have zero bytes in cold storage.

Required file: hostinfo.conf
		This file stores the master server to query, and the username/password combo to use for authentication.

Format of hostinfo.config:
	A file with one entry per line:
		master server url, e.g.: https://master.example.com:4285
		username
		password

Output:
	1. log file: one date-stamped log file per calendar day. Multiple runs on the same day append to this file.
	2. Date-stamped CSV file with list of archives with changed purge date. Fields: Archive GUID, Old Purge Date, New Purge Date
		When run in test mode, the CSV file has the prefix "test_"
	3. Console output similar to what is in the log file

Misc. Notes:
	When the baseline date is given as a date (format = MM-DD-YYYY), the time zone associated with the resultant date
	object is UTC or GMT. When the date is specified with TODAY, the associated date object is the time zone of the
	host machine. This produces an unintended effect: if you run the program more than once with the same baseline date
	(not specified as TODAY), the program will keep rewriting the new purge date with PUT calls to ColdStorage API, because
	it thinks the new purge date is GMT, whereas the purge date stored in the server is the time zone of the server.
	Since users will not need to run more than once with a particular new archive expiration date, I don't think it's
	worth spending time to fix at this time.

Version 1.0
