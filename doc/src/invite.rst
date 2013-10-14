Invitation Process
==================

1. .. http:post:: /api/friends

	**Request**: NewFriend object

	.. code-block:: json

		{
		    "passport": "cZZg9SqrCyI_QvXNC_WPPosf4lxU-sFDlBviwhJycy5oMHRiMHgubmV0OjIxMzQ="
		}

	**Response**: Friend object

	.. code-block:: json

		{
		    "fid": "m8OKHgBzOgHxX2Q0wCU5nwK5qK2VJpKglHFTDg==",
		    "host": "66.235.47.86",
		    "port": 32777
		}

2. .. http:put:: /api/friends/{fid}/outbox/{cname}

	:param fid:   Friend fingerprint
	:param cname: Collection name

	**Request**: Invite object

	.. code-block:: json

		{
		    "cid": "ff1O1yvhdc2dtyPFNSR5QVNhSWCf7fCAGxIUEQ==",
		    "name": "profile",
		    "description": "fried's Profile",
		    "icon": "/api/friends/m8OKHgBzOgHxX2Q0wCU5nwK5qK2VJpKglHFTDg==/outbox/profile/picture.png"
		}

3. .. http:get:: /api/inbox?state=eq=pending

	:query state: Filter by ``state == pending``

	**Response**: Invite object

	.. code-block:: json

		{
		    "cid": "ff1O1yvhdc2dtyPFNSR5QVNhSWCf7fCAGxIUEQ==",
		    "name": "profile",
		    "description": "fried's Profile",
		    "icon": "/api/friends/m8OKHgBzOgHxX2Q0wCU5nwK5qK2VJpKglHFTDg==/outbox/profile/picture.png",
		    "state": "pending"
		}

4. .. http:post:: /api/
