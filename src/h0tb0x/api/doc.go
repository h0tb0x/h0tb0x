/*

[		]

***********************
Self
***********************

The currently authenticated user profile represents a single [PHRASING].

/api/self		

	GET		return information about the currently authenticated local user profile
			returns: selfJson





***********************
Friends 
***********************
	
A friendship is a mutual relation between two profiles. Each user profile stores information corresponding to the other user as a friend object.

/api/friends		

	GET		Get friends associated with currently authenticated local user profile.

			returns: collection of friendJson elements

	POST		Add friend association to local user profile.
			request body: 	friendJson corresponding to friend to be added
			returns:	n/a


/api/friends/{who}	{who} -- fingerprint identifying the friend 

	GET		Get information about the friend identified by the given fingerprint.
      			returns: a json-encoded structure 

	PUT		[	definition	]
			request body:	n/a
			returns: 	n/a

	DELETE		Deletes friend with the given fingerprint.
			request body:  n/a
			returns:       n/a


***********************
Collections
***********************

A collection is

/api/collections	

	GET		Get all collections associated with currently authenticated user profile.


	POST		Add a collection.

/api/collections/{cid}

	GET		Get a particular collection belonging to local user profile.
			returns: collection corresponding to {cid}




***********************
Collection writers
***********************


/api/collections/{cid}/writers

	GET		Get all writers authorized to write to a collection.
			returns: writers for collection corresponding to {cid}

	POST		Add a writer to a collection.
			

/api/collections/{cid}/writers/{who}	

	GET		[		]



	DELETE		[		]





***********************
Collection data
***********************


/api/collections/{cid}/data

	GET		Get data for a collection.


/api/collections/{cid}/data/{key:.+}	

	GET		[	definition	]
			returns:

	PUT		[	definition	]
			request body:
			
	POST		
			request body:

	DELETE		not implemented
			




***********************
Invitations
***********************


/api/invites

POST		Post an invitation.
		request body: inviteJson

*/

package api
