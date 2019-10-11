# Mattermost Giphy Plugin

This plugin bring the magic of Giphy translate to Mattermost. For a stable production release, please download the latest version [in the Releases tab](https://github.com/mattermost/mattermost-plugin-giphy/releases) and follow [these instructions](#2-configuration) for install and configuration.

## Table of Contents

 - [1. Users](#1-users).
 - [2. System administrators](#2-system-administrators).
 - [3. Developers](#3-developers).
 - [4. FAQ](#4-frequently-asked-questions-faq).
 - [5. Help](#5-help).
 
### Requirements
- For Giphy Plugin 1.0, Mattermost Server v5.14+ is required
- Giphy plugin requires configuring a [Giphy API Key](https://developers.giphy.com/faq). 

## 1. Users

The giphy plugin implements a `/giphy` command which can be used to look up and post images for phrases.
It uses the [Giphy Translate API](https://developers.giphy.com/docs/api/endpoint#translate).

Usage:
```
/giphy <some search query>
```

- For instance, typing `/giphy Hello, strange world of giphy!` will display,
<img src="https://user-images.githubusercontent.com/1187448/63696085-cf806780-c7ce-11e9-9c77-a4fa8c693bf0.png" width="500"/>

This is presented only to the user who typed the message and is not posted to the channel, yet.

- To get a different image from Giphy, try "Shuffle",
<img src="https://user-images.githubusercontent.com/1187448/63696144-f0e15380-c7ce-11e9-9949-6aced7b29a51.png" width="500"/>


- To post to the channel, press "Send",
<img src="https://user-images.githubusercontent.com/1187448/63696271-3140d180-c7cf-11e9-8a77-f93c9868e9ae.png" width="500"/>

At this point, the image appears in the channel, and all participants see it as a new message from the user.

## 2. System administrators

1. Go to **System Console > Plugins > Giphy**, and enter your [Giphy API Key](https://developers.giphy.com/faq), the desired MPAA rating (Y, G, PG, PG-13, R, Unrated, or NSFW), and the weirdness value (randomness) of GIF suggestions (0-10 values)
2. Go to **System Console > Plugins > Management** and click **Enable** to enable the Giphy plugin.

## 3. Developers

- TODO

Use `make dist` to build distributions of the plugin that you can upload to a Mattermost server.
Use `make all` to run all checks and build.
Use `make deploy` to deploy the plugin to your local server.

For additional information on developing plugins, refer to [our plugin developer documentation](https://developers.mattermost.com/extend/plugins/).

## 4. Frequently Asked Questions (FAQ)

### How do I disable the plugin quickly in an emergency?

Disable the Giphy plugin any time from **System Console > Plugins > Management**. 

## 5. Help

For Mattermost customers - please open a support case.
For Questions, Suggestions and Help - please find us on our forum at https://forum.mattermost.org/c/plugins
To Contribute to the project see https://www.mattermost.org/contribute-to-mattermost/
