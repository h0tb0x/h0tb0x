Invitation Process
==================

1. .. http:post:: /api/friends

	Begin friending process.
	
	**Request**: NewFriend object

	.. code-block:: http

		POST /api/friends HTTP/1.1
		Accept: application/json
		Content-Type: application/json

		{
		    "passport": "cZZg9SqrCyI_QvXNC_WPPosf4lxU-sFDlBviwhJycy5oMHRiMHgubmV0OjIxMzQ="
		}

	**Response**: Friend object

	.. code-block:: http

		HTTP/1.1 200 OK
		Cache-Control: must-revalidate
		Content-Type: application/json
		ETag: "..."

		{
		    "fid": "m8OKHgBzOgHxX2Q0wCU5nwK5qK2VJpKglHFTDg==",
		    "host": "66.235.47.86",
		    "port": 32777
		}

3. .. http:post:: /api/friends/{fid}/outbox

	:param fid:   Friend fingerprint

	**Request**: Invite object

	.. code-block:: http

		POST /api/friends HTTP/1.1
		Accept: application/json
		Content-Type: application/json

		{
		    "key": "profile",
		    "type": "invite",
		    "cid": "@collection:ff1O1yvhdc2dtyPFNSR5QVNhSWCf7fCAGxIUEQ==",
		    "description": "fried's Profile",
		    "icon": "@attach:<hash>"
		}

3. .. http:get:: /api/inbox?type=invite&dirty=true

	:query type: Filter by ``type == "invite"``
	:query dirty: Filter by ``dirty == true``

	**Response**: Invite object

	.. code-block:: http

		GET /api/inbox?type=invite&dirty=true HTTP/1.1
		Accept: application/json

		{
		    "who": "<friend_fid>",
		    "where": "<friend_inbox_cid>/data/profile",
		    "what": {
		        "key": "profile",
		        "type": "invite",
		        "cid": "@collection:ff1O1yvhdc2dtyPFNSR5QVNhSWCf7fCAGxIUEQ==",
		        "description": "fried's Profile",
		        "icon": "@attach:<hash>",
		        "notes": {
		        	"dirty": true
		        }
		    }
		}

4. .. http:post:: /api/
