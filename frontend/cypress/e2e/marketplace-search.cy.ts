const NULL_UUID = '00000000-0000-0000-0000-000000000000';

const DESK_LISTING = {
  id: 'listing-desk',
  seller_id: 'seller-1',
  title: 'Standing Desk',
  description: 'An adjustable desk.',
  price: 80,
  image_count: 0,
  first_image_id: null,
  seller_name: 'Test Seller',
};

describe('Marketplace Search', () => {
  let listingRequestCount: number;

  beforeEach(() => {
    listingRequestCount = 0;

    cy.intercept('GET', '/api/users/me', {
      statusCode: 200,
      body: {
        id: 'user-1',
        first_name: 'Test',
        last_name: 'User',
        email: 'testuser@ufl.edu',
        image_id: null,
      },
    }).as('authCheck');

    cy.intercept('GET', '/api/listings?*', (request) => {
      listingRequestCount++;
      const body = request.query['query'] === 'desk' ? [DESK_LISTING] : [];
      request.reply({ statusCode: 200, body });
    }).as('getListings');

    cy.visit('/main');
    cy.wait('@authCheck');
    cy.wait('@getListings');
  });

  it('does not request listings while the user is typing', () => {
    cy.get('input[placeholder="Search marketplace..."]').type('desk');

    cy.then(() => {
      expect(listingRequestCount).to.equal(1);
    });
  });

  it('searches by query when the user presses Enter', () => {
    cy.get('input[placeholder="Search marketplace..."]').type('desk{enter}');

    cy.wait('@getListings').then(({ request }) => {
      expect(request.query).to.include({
        query: 'desk',
        limit: '20',
        cursor: NULL_UUID,
      });
      expect(request.query).not.to.have.property('key');
    });
    cy.contains('Standing Desk').should('be.visible');
  });

  it('searches when the user clicks the Search button', () => {
    cy.get('input[placeholder="Search marketplace..."]').type('desk');
    cy.get('button.search-button').click();

    cy.wait('@getListings').then(({ request }) => {
      expect(request.query['query']).to.equal('desk');
      expect(request.query).not.to.have.property('key');
    });
    cy.contains('Standing Desk').should('be.visible');
  });
});
