/*

H0tb0x gives users a way to share collections of data with trusted peers. 
Sharing occurs across network links represented by friend relationships.
To establish a friendship, two parties send introductory passports to each other.
Once passports have been exchanged, a network link is maintained automatically.
Users curate data organized into collections, and each user may authorize their friends to view or modify a collection.
Users communicate changes to collections via signed messages. A change is valid if its author belongs to the group of authorized writers for that collection.


***********************
Self
***********************

Self identifies the currently authenticated profile. Users may have many profiles.

/api/self

	GET		Provides information about the currently authenticated local user profile.
			returns: json-encoded object representing the current profile.. 




***********************
Friends
***********************

A friendship is a mutual relation between two profiles. Each user profile stores information corresponding to the other user as a friend object.

/api/friends

	GET		Get friends associated with currently authenticated local user profile.
			returns: collection of json-encoded friend objects.

	POST		Add friend association to local user profile.
			request body: 	json-encoded friend object corresponding to new friend.


/api/friends/{who}	
	Data types:	{who} -- fingerprint identifying the friend

	GET		Get information about the friend identified by the given fingerprint.
      			returns: a json-encoded structure

	PUT		Add information about the friend idetnified by the given fingerprint.
			request body:	n/a
			returns: 	n/a

	DELETE		Deletes friend with the given fingerprint.
			request body:  n/a
			returns:       n/a


***********************
Collections
***********************

A collection is a curated set of data with common authorized writers and authorized viewers.

/api/collections

	GET		Get all collections associated with currently authenticated user profile.
			returns: json-encoded set of objects representing all available collections

	POST		Add a collection.

/api/collections/{cid}

	Data types:	{cid} -- string representing the collection id.
	     		{who} -- fingerprint identifying a friend

	GET		Get a particular collection belonging to local user profile.
			returns: json-encoded object representing the collection labeled {cid}



***********************
Collection writers
***********************


/api/collections/{cid}/writers

	GET		Get all writers authorized to write to a collection.
			returns: json-encoded set of friends authorized to write to collection labeled {cid}

	POST		Authorize a friend as a writer to a collection.
			request body: json-encoded object representing the friend

/api/collections/{cid}/writers/{who}

	Data types:	{cid} -- string representing the collection id.
	     		
	GET		Get information about a writer associated with a particular collection.
			returns: json-encoded object representing a friend authorized to write to collection labeled {cid}


	DELETE		Remove a friend from the set of writers authorized to modify a particular collection.





***********************
Collection data
***********************

[	Definition	]

/api/collections/{cid}/data

	GET		Get data for a collection.
			returns:


/api/collections/{cid}/data/{key:.+}

	GET		Get collection data for a particular data element.
			returns:

	PUT		
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
