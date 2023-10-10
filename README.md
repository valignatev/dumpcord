# Dumpcord

Simple program to dump attachments from discord channel due to Discord's recent addition of expiration date to their CDN urls.
It's a bit barebons for now, but does the job. Maybe I'll improve it later, or maybe I'll just dump channels I'm interested in
and abandon it forever.

No discord bot required, only your (the user's) Auth token. There are multiple tutorials on how to get it.
You just open discord's debug panel and grab the auth header form any request in the Network tab.

The code is Free And Opensource (tm) so you see that I'm not stealing it.

RIP abusing free VC money :(

# Run
Run `dumpcord` without arguments to see the usage
it creates directory next to the executable where it downloads everything.

# TODO
Parse discord CDN urls in the post content, for now only attachments are getting downloaded

# LICENSE
MIT
