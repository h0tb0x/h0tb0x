Invitation Process
==================

.. glossary:: System Attributes
	
	``_id``
		Record ID

	``_dirty``
		Dirty flag, denotes whether a record has been recently updated.

	``_attachments``
		Attachments for a record

	``_notes``
		Local notes

Attachment object

.. code-block:: json 

	{
	    "photo.jpg": {
	        "content_type": "image/jpeg",
	        "digest": "sha1-",
	        "length": 165504
	    }
	}

1. .. http:post:: /api/friends

	Begin friending process.
	
	**Request JSON Object**: Friend request object
	 	* passport (*string*) - The passport from a new friend

	.. code-block:: http

		POST /api/friends HTTP/1.1
		Accept: application/json
		Content-Type: application/json
		Host: localhost:8000

		{
		    "passport": "cZZg9SqrCyI_QvXNC_WPPosf4lxU-sFDlBviwhJycy5oMHRiMHgubmV0OjIxMzQ="
		}

	**Response JSON Object**: Friend object
		* fid (*string*) - Friend ID
		* host (*string*)
		* port (*number*)

	.. code-block:: http

		HTTP/1.1 200 OK
		Cache-Control: must-revalidate
		Content-Type: application/json
		Server: h0tb0x

		{
		    "fid": "cZZg9SqrCyI_QvXNC_WPPosf4lxU-sFDlBviwg==",
		    "host": "66.235.47.86",
		    "port": 32777
		}

3. .. http:post:: /api/friends/{fid}/outbox

	:param string fid: Friend fingerprint

	**Request JSON Object**: Invite object

	.. code-block:: http

		POST /api/friends HTTP/1.1
		Accept: application/json
		Content-Type: application/json
		Host: localhost:8000

		{
		    "_invite": "60XfetbRZk09ZyBhmjEn9Y5J2S5Y11p62KibmA==",
		    "name": "profile",
		    "description": "fried's Profile",
		    "_attachments": {
		        "icon": "<hash of attachment>"
		    }
		}

	**Reponse JSON Object**: Status object

	.. code-block:: http

		HTTP/1.1 200 OK
		Cache-Control: must-revalidate
		Content-Type: application/json
		Server: h0tb0x

		{
		    "ok": "true",
		    "_id": "..."
		}

3. .. http:get:: /api/invites?_dirty=true

	:query boolean _dirty: Filter by ``dirty == true``

	**Request**

	.. code-block:: http

		GET /api/invites?_dirty=true HTTP/1.1
		Accept: application/json
		Host: localhost:8000

	**Response JSON Object**: Invite object

	.. code-block:: http

		HTTP/1.1 200 OK
		Cache-Control: must-revalidate
		Content-Type: application/json
		Server: h0tb0x

		{
		    "_id": "...",
		    "_fid": "m8OKHgBzOgHxX2Q0wCU5nwK5qK2VJpKglHFTDg==",
		    "_invite": "60XfetbRZk09ZyBhmjEn9Y5J2S5Y11p62KibmA==",
		    "_dirty": true,
		    "description": "fried's Profile",
		    "_attachments": {
		        "icon": "<hash of attachment>"
		    }
		}

4. .. http:post:: /api/invites

	:query boolean _accept: Accept flag

	Accept

	**Request JSON Object** Invite object

	.. code-block:: http

		POST /api/invites?accept HTTP/1.1
		Accept: application/json
		Host: localhost:8000

		{
		    "_id": "...",
		    "_fid": "m8OKHgBzOgHxX2Q0wCU5nwK5qK2VJpKglHFTDg==",
		    "_invite": "60XfetbRZk09ZyBhmjEn9Y5J2S5Y11p62KibmA==",
		    "_dirty": false,
		    "_notes": {
		    	"disposition": "accepted"
		    }
		}

	Reject

	**Request JSON Object** Invite object

	.. code-block:: http

		POST /api/invites HTTP/1.1
		Accept: application/json
		Host: localhost:8000

		{
		    "_id": "...",
		    "_fid": "m8OKHgBzOgHxX2Q0wCU5nwK5qK2VJpKglHFTDg==",
		    "_invite": "60XfetbRZk09ZyBhmjEn9Y5J2S5Y11p62KibmA==",
		    "_dirty": false,
		    "_notes": {
		    	"disposition": "rejected"
		    }
		}

	Ignore

	**Request JSON Object** Invite object

	.. code-block:: http

		POST /api/invites HTTP/1.1
		Accept: application/json
		Host: localhost:8000

		{
		    "_id": "...",
		    "_fid": "m8OKHgBzOgHxX2Q0wCU5nwK5qK2VJpKglHFTDg==",
		    "_invite": "60XfetbRZk09ZyBhmjEn9Y5J2S5Y11p62KibmA==",
		    "_dirty": false,
		    "_notes": {
		    	"disposition": "ignored"
		    }
		}
